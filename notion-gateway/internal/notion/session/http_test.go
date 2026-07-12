package session

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/notioncodex/gateway/internal/notion/discovery/picker"
)

type staticCredentials struct{ value string }

func (s staticCredentials) CookieHeader(context.Context) ([]byte, error) {
	return []byte(s.value), nil
}

func TestSessionDiscoveryRequest(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != picker.ObservedPath {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Cookie") != "session=local-test-value" {
			t.Fatal("credential was not injected")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"spaceId":"workspace"}` {
			t.Fatalf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"model":"dynamic","modelMessage":"Dynamic"}]}`)
	}))
	defer server.Close()

	session, err := New(staticCredentials{value: "session=local-test-value"})
	if err != nil {
		t.Fatal(err)
	}
	session.origin, _ = url.Parse(server.URL)
	response, err := session.Do(context.Background(), picker.Request{Method: http.MethodPost, Path: picker.ObservedPath, Body: []byte(`{"spaceId":"workspace"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(response), "dynamic") {
		t.Fatalf("response = %s", response)
	}
}

func TestSessionMapsStatusWithoutBodyLeak(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "private upstream body")
	}))
	defer server.Close()
	session, _ := New(staticCredentials{value: "session=test"})
	session.origin, _ = url.Parse(server.URL)
	_, err := session.Do(context.Background(), picker.Request{Method: http.MethodPost, Path: picker.ObservedPath, Body: []byte(`{}`)})
	var httpError *HTTPError
	if !errors.As(err, &httpError) || httpError.Category != "credentials_refresh_required" {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "private upstream body") {
		t.Fatal("response body leaked through error")
	}
}

func TestSessionRejectsUnobservedPath(t *testing.T) {
	t.Parallel()
	session, _ := New(staticCredentials{value: "session=test"})
	_, err := session.Do(context.Background(), picker.Request{Method: http.MethodPost, Path: "/unobserved", Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected path rejection")
	}
}
