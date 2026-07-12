// Package session provides the authenticated HTTP boundary used by observed
// Notion adapters. It never logs or persists credentials.
package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/notioncodex/gateway/internal/notion/discovery/picker"
)

const (
	defaultOrigin = "https://" + "app." + "notion.com"
	maxBodyBytes  = 2 << 20
)

type CredentialSource interface {
	CookieHeader(context.Context) ([]byte, error)
}

type HTTPError struct {
	StatusCode int
	Category   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("notion request failed: %s", e.Category) }

type Session struct {
	origin      *url.URL
	client      *http.Client
	credentials CredentialSource
}

func New(credentials CredentialSource) (*Session, error) {
	if credentials == nil {
		return nil, errors.New("credential source is required")
	}
	origin, err := url.Parse(defaultOrigin)
	if err != nil {
		return nil, err
	}
	return &Session{
		origin:      origin,
		client:      &http.Client{Timeout: 45 * time.Second},
		credentials: credentials,
	}, nil
}

func (s *Session) Do(ctx context.Context, request picker.Request) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Method != http.MethodPost {
		return nil, errors.New("observed session permits POST only")
	}
	if request.Path != picker.ObservedPath {
		return nil, errors.New("request path is not an observed adapter path")
	}
	if len(request.Body) > maxBodyBytes {
		return nil, errors.New("request body exceeds limit")
	}
	cookie, err := s.credentials.CookieHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("load session credential: %w", err)
	}
	defer zero(cookie)
	if len(bytes.TrimSpace(cookie)) == 0 {
		return nil, errors.New("session credential is empty")
	}

	target := s.origin.ResolveReference(&url.URL{Path: request.Path})
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target.String(), bytes.NewReader(request.Body))
	if err != nil {
		return nil, fmt.Errorf("build notion request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Cookie", string(cookie))

	response, err := s.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("execute notion request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return nil, &HTTPError{StatusCode: response.StatusCode, Category: statusCategory(response.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read notion response: %w", err)
	}
	if len(body) > maxBodyBytes {
		return nil, errors.New("notion response exceeds limit")
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/json") {
		return nil, errors.New("notion response is not JSON")
	}
	return body, nil
}

func statusCategory(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "credentials_refresh_required"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if status >= 500 {
			return "upstream_unavailable"
		}
		return "upstream_rejected"
	}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
