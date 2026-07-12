package picker_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/notioncodex/gateway/internal/notion/discovery/picker"
	"github.com/notioncodex/gateway/internal/notion/fixtures"
)

type fakeSession struct {
	request  picker.Request
	response []byte
}

func (s *fakeSession) Do(_ context.Context, request picker.Request) ([]byte, error) {
	s.request = request
	return s.response, nil
}

func TestDiscoverObservedContract(t *testing.T) {
	t.Parallel()
	file, err := os.Open(filepath.Join("..", "..", "..", "..", "testdata", "notion", "model-picker.observed.redacted.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	envelope, err := fixtures.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	session := &fakeSession{response: envelope.Payload}
	observedAt := time.Date(2026, 7, 11, 8, 55, 57, 0, time.UTC)
	discoverer := &picker.Discoverer{Session: session, Now: func() time.Time { return observedAt }}
	models, err := discoverer.Discover(context.Background(), "account", "workspace")
	if err != nil {
		t.Fatal(err)
	}
	if session.request.Method != "POST" || session.request.Path != picker.ObservedPath {
		t.Fatalf("unexpected request: %#v", session.request)
	}
	var body map[string]string
	if err := json.Unmarshal(session.request.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["spaceId"] != "workspace" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if len(models) != 3 {
		t.Fatalf("models = %d, want 3", len(models))
	}
	for _, model := range models {
		if model.PublicID != "notion/"+model.UpstreamID {
			t.Fatalf("public ID was not dynamically derived: %#v", model)
		}
		if !model.Capabilities.Text || model.Capabilities.Vision || model.Capabilities.Tools || model.Capabilities.WebSearch || model.Capabilities.ReasoningSummary {
			t.Fatalf("unsupported capability inferred: %#v", model.Capabilities)
		}
		if model.Status != "available" || !model.LastVerifiedAt.Equal(observedAt) {
			t.Fatalf("unexpected status/timestamp: %#v", model)
		}
	}
}

func TestParseRejectsDuplicateModel(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"models":[{"model":"same","modelMessage":"One"},{"model":"same","modelMessage":"Two"}]}`)
	if _, err := picker.Parse(payload, "account", "workspace", time.Now()); err == nil {
		t.Fatal("expected duplicate model error")
	}
}

func TestParseDisabledModel(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"models":[{"model":"disabled","modelMessage":"Disabled","isDisabled":true}]}`)
	models, err := picker.Parse(payload, "account", "workspace", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if models[0].Status != "disabled" {
		t.Fatalf("status = %q", models[0].Status)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"models":[{"model":"id","modelMessage":"Name","isDisabled":false}]}`))
	f.Add([]byte(`{"models":[]}`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 2<<20 {
			t.Skip()
		}
		_, _ = picker.Parse(payload, "account", "workspace", time.Unix(0, 0))
	})
}
