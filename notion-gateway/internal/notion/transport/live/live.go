package live

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/notioncodex/gateway/internal/domain"
	"github.com/notioncodex/gateway/internal/notion/discovery/picker"
	"github.com/notioncodex/gateway/internal/notion/inference/transcript"
	"github.com/notioncodex/gateway/internal/ports"
)

type Transport struct {
	origin *url.URL
	client *http.Client
	cookie []byte
	spaceID string
	timezone string
	mu sync.RWMutex
	defaultModel string
}

func debugEnabled() bool {
	return strings.EqualFold(os.Getenv("NOTION_CODEX_DEBUG"), "true") || os.Getenv("NOTION_CODEX_DEBUG") == "1"
}

func debugf(format string, args ...any) {
	if debugEnabled() {
		log.Printf("[live] "+format, args...)
	}
}

func previewBytes(b []byte, limit int) string {
	if len(b) == 0 {
		return "<empty>"
	}
	if len(b) > limit {
		return strings.TrimSpace(string(b[:limit])) + "…"
	}
	return strings.TrimSpace(string(b))
}

func New(spaceID string, cookie []byte, timezone string) (*Transport, error) {
	if strings.TrimSpace(spaceID) == "" || len(bytes.TrimSpace(cookie)) == 0 { return nil, errors.New("space ID and session cookie are required") }
	origin, err := url.Parse("https://" + "app." + "notion.com")
	if err != nil { return nil, err }
	return &Transport{origin: origin, client: &http.Client{Timeout: 5*time.Minute}, cookie: append([]byte(nil), cookie...), spaceID: spaceID, timezone: timezone}, nil
}

func (t *Transport) Discover(ctx context.Context, accountID, workspaceID string) ([]domain.Model, error) {
	body, err := json.Marshal(struct { SpaceID string `json:"spaceId"` }{SpaceID: t.spaceID})
	if err != nil { return nil, err }
	debugf("discover request space_id=%s payload=%s", t.spaceID, previewBytes(body, 800))
	response, err := t.request(ctx, picker.ObservedPath, body, "application/json")
	if err != nil { return nil, err }
	defer response.Close()
	payload, err := io.ReadAll(io.LimitReader(response, 2<<20))
	if err != nil { return nil, err }
	debugf("discover response bytes=%d preview=%s", len(payload), previewBytes(payload, 1200))
	models, err := picker.Parse(payload, accountID, workspaceID, time.Now())
	if err != nil { return nil, err }
	for _, model := range models { if model.Status == "available" { t.mu.Lock(); t.defaultModel = model.UpstreamID; t.mu.Unlock(); break } }
	debugf("discover parsed models=%d default_model=%s", len(models), t.defaultModel)
	return models, nil
}

func (t *Transport) Infer(ctx context.Context, request domain.InferenceRequest) (ports.EventStream, error) {
	model := strings.TrimPrefix(request.Model, "notion/")
	if request.Model == "notion-auto" { t.mu.RLock(); model = t.defaultModel; t.mu.RUnlock() }
	if model == "" { return nil, errors.New("no available model selected") }
	variants, err := transcript.BuildVariants(transcript.BuildOptions{SpaceID: t.spaceID, Model: model, Input: request.Input, Timezone: t.timezone})
	if err != nil { return nil, err }
	var lastErr error
	for _, variant := range variants {
		debugf("infer attempt variant=%s model=%s input_len=%d input_preview=%q payload=%s", variant.Name, model, len(request.Input), previewBytes([]byte(request.Input), 300), previewBytes(variant.Payload, 1600))
		response, err := t.request(ctx, transcript.ObservedPath, variant.Payload, "application/x-ndjson")
		if err != nil {
			lastErr = fmt.Errorf("variant %s: %w", variant.Name, err)
			debugf("infer variant=%s request error=%v", variant.Name, err)
			continue
		}
		prepared, empty, err := prepareResponseStream(response)
		if err != nil {
			lastErr = fmt.Errorf("variant %s: %w", variant.Name, err)
			debugf("infer variant=%s stream prep error=%v", variant.Name, err)
			continue
		}
		if empty {
			lastErr = fmt.Errorf("variant %s returned empty stream", variant.Name)
			debugf("infer variant=%s returned empty stream preview=%q", variant.Name, prepared.preview)
			continue
		}
		debugf("infer stream opened model=%s variant=%s preview=%q", model, variant.Name, prepared.preview)
		return &stream{body: prepared.body, decoder: transcript.NewDecoder(prepared.reader)}, nil
	}
	if lastErr != nil { return nil, lastErr }
	return nil, errors.New("all inference variants failed")
}

type preparedStream struct {
	reader  io.Reader
	body    io.ReadCloser
	preview string
}

func prepareResponseStream(body io.ReadCloser) (preparedStream, bool, error) {
	reader := bufio.NewReader(body)
	var consumed bytes.Buffer
	for consumed.Len() < 64 {
		b, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				preview := strings.TrimSpace(consumed.String())
				if preview == "" || preview == "[]" {
					_ = body.Close()
					return preparedStream{preview: preview}, true, nil
				}
				return preparedStream{reader: bytes.NewReader(consumed.Bytes()), body: body, preview: preview}, false, nil
			}
			_ = body.Close()
			return preparedStream{}, false, err
		}
		consumed.WriteByte(b)
		trimmed := strings.TrimSpace(consumed.String())
		if trimmed == "" || trimmed == "[" {
			continue
		}
		if trimmed == "[]" {
			_ = body.Close()
			return preparedStream{preview: trimmed}, true, nil
		}
		return preparedStream{reader: io.MultiReader(bytes.NewReader(consumed.Bytes()), reader), body: body, preview: trimmed}, false, nil
	}
	trimmed := strings.TrimSpace(consumed.String())
	if trimmed == "[]" {
		_ = body.Close()
		return preparedStream{preview: trimmed}, true, nil
	}
	return preparedStream{reader: io.MultiReader(bytes.NewReader(consumed.Bytes()), reader), body: body, preview: trimmed}, false, nil
}

func (t *Transport) request(ctx context.Context, path string, body []byte, expectedType string) (io.ReadCloser, error) {
	target := t.origin.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil { return nil, err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", expectedType)
	req.Header.Set("Cookie", string(t.cookie))
	debugf("http request method=%s path=%s accept=%s cookie_len=%d body=%s", req.Method, path, expectedType, len(t.cookie), previewBytes(body, 1600))
	response, err := t.client.Do(req)
	if err != nil { return nil, fmt.Errorf("notion request failed: %w", err) }
	debugf("http response path=%s status=%s content_type=%q content_length=%d", path, response.Status, response.Header.Get("Content-Type"), response.ContentLength)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
		preview := readBodyPreview(response.Body, 8192)
		debugf("http rejected path=%s status=%s body=%s", path, response.Status, preview)
		return nil, fmt.Errorf("notion request rejected: status=%s content_type=%q body=%s", response.Status, contentType, preview)
	}
	contentType := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Type")))
	if acceptsContentType(contentType, expectedType) { return response.Body, nil }
	preview := readBodyPreview(response.Body, 8192)
	debugf("http unexpected content type path=%s got=%q want=%q body=%s", path, contentType, expectedType, preview)
	return nil, fmt.Errorf("unexpected notion content type: got=%q want=%q body=%s", contentType, expectedType, preview)
}

func acceptsContentType(contentType, expectedType string) bool {
	expectedType = strings.ToLower(strings.TrimSpace(expectedType))
	if strings.Contains(contentType, expectedType) {
		return true
	}
	if expectedType == "application/x-ndjson" {
		return strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/plain")
	}
	return false
}

func readBodyPreview(body io.ReadCloser, limit int64) string {
	if body == nil {
		return "<nil>"
	}
	defer body.Close()
	payload, err := io.ReadAll(io.LimitReader(body, limit))
	if err != nil {
		return fmt.Sprintf("<read error: %v>", err)
	}
	preview := strings.TrimSpace(string(payload))
	if preview == "" {
		return "<empty>"
	}
	return preview
}

type stream struct { body io.ReadCloser; decoder *transcript.Decoder }
func (s *stream) Recv(ctx context.Context) (domain.InferenceEvent, error) { return s.decoder.Next(ctx) }
func (s *stream) Close() error { return s.body.Close() }
