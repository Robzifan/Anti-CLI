package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NOTION_CODEX_ADDRESS", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected empty address error")
	}
}
