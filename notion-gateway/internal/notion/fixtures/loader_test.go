package fixtures

import (
	"errors"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{name: "valid", input: `{"schema_version":1,"direction":"event","transport":"unknown","payload":{"type":"<redacted>"}}`},
		{name: "unknown envelope field", input: `{"schema_version":1,"direction":"event","transport":"unknown","payload":{},"extra":true}`, wantError: true},
		{name: "forbidden nested cookie", input: `{"schema_version":1,"direction":"event","transport":"unknown","payload":{"headers":{"cookie":"secret"}}}`, wantError: true},
		{name: "bearer value", input: `{"schema_version":1,"direction":"event","transport":"unknown","payload":{"value":"Bearer abcdefghijklmnop"}}`, wantError: true},
		{name: "jwt value", input: `{"schema_version":1,"direction":"event","transport":"unknown","payload":{"value":"eyJabcde.abcdefgh.ijklmnop"}}`, wantError: true},
		{name: "invalid direction", input: `{"schema_version":1,"direction":"sideways","transport":"unknown","payload":{}}`, wantError: true},
		{name: "trailing document", input: `{"schema_version":1,"direction":"event","transport":"unknown","payload":{}} {}`, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(strings.NewReader(test.input))
			if (err != nil) != test.wantError {
				t.Fatalf("Load() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}

func TestLoadTooLarge(t *testing.T) {
	t.Parallel()
	_, err := Load(strings.NewReader(strings.Repeat("x", int(MaxFixtureBytes)+1)))
	if !errors.Is(err, errTooLarge) {
		t.Fatalf("Load() error = %v, want %v", err, errTooLarge)
	}
}

func FuzzLoad(f *testing.F) {
	f.Add([]byte(`{"schema_version":1,"direction":"event","transport":"unknown","payload":{}}`))
	f.Add([]byte(`{"cookie":"secret"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if int64(len(data)) > MaxFixtureBytes+1 {
			t.Skip()
		}
		_, _ = Load(strings.NewReader(string(data)))
	})
}
