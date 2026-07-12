package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProbeFixture(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	good := filepath.Join(directory, "good.json")
	if err := os.WriteFile(good, []byte(`{"schema_version":1,"direction":"response","transport":"unknown","payload":{"event":"redacted"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"probe-fixture", good}); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(directory, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"schema_version":1,"direction":"response","transport":"unknown","payload":{"cookie":"secret"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"probe-fixture", bad}); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestDecodeNormalizedStream(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "stream.ndjson")
	if err := os.WriteFile(path, []byte("{\"kind\":\"delta\",\"text\":\"hello\"}\n{\"kind\":\"done\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"decode-normalized-stream", path}); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsDirectory(t *testing.T) {
	t.Parallel()
	if err := run([]string{"probe-fixture", t.TempDir()}); err == nil {
		t.Fatal("expected directory rejection")
	}
}
