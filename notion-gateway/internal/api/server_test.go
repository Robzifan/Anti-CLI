package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notioncodex/gateway/internal/api"
	"github.com/notioncodex/gateway/internal/models"
	"github.com/notioncodex/gateway/internal/notion/transport/mock"
)

func handler(t *testing.T) http.Handler {
	t.Helper()
	tr := &mock.Transport{}
	r := models.NewRegistry(tr)
	if err := r.Refresh(context.Background(), "a", "w"); err != nil {
		t.Fatal(err)
	}
	return api.New(slog.New(slog.NewTextHandler(io.Discard, nil)), r, tr, time.Second)
}
func TestHealthAndModels(t *testing.T) {
	h := handler(t)
	for _, tc := range []struct {
		path string
		code int
	}{{"/healthz", 200}, {"/readyz", 200}, {"/v1/models", 200}} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rr.Code != tc.code {
			t.Fatalf("%s: got %d", tc.path, rr.Code)
		}
	}
}
func TestResponsesNonStreaming(t *testing.T) {
	rr := httptest.NewRecorder()
	h := handler(t)
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"mock/echo","input":"hello","stream":false}`)))
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["status"] != "completed" {
		t.Fatalf("unexpected response: %#v", got)
	}
}
func TestResponsesStreaming(t *testing.T) {
	srv := httptest.NewServer(handler(t))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", bytes.NewBufferString(`{"model":"mock/echo","input":"Привет 🌍","stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type=%q", resp.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(resp.Body)
	var all strings.Builder
	for scanner.Scan() {
		all.WriteString(scanner.Text())
		all.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	body := all.String()
	for _, event := range []string{"response.created", "response.output_item.added", "response.content_part.added", "response.output_text.delta", "response.output_text.done", "response.output_item.done", "response.completed"} {
		if !strings.Contains(body, "event: "+event) {
			t.Errorf("missing %s in %s", event, body)
		}
	}
	if !strings.Contains(body, "Привет 🌍") {
		t.Fatal("unicode text missing")
	}
}
func TestInvalidRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	handler(t).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"mock/echo","input":{"bad":true}}`)))
	if rr.Code != 400 {
		t.Fatalf("status=%d", rr.Code)
	}
}
