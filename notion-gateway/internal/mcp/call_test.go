package mcp

import "testing"

func TestToolAllowed(t *testing.T) {
    config := ServerConfig{AllowedTools: []string{"read_text_file"}}
    if !toolAllowed(config, "read_text_file") {
        t.Fatal("expected configured tool to be allowed")
    }
    if toolAllowed(config, "write_file") {
        t.Fatal("unexpected write tool allowance")
    }
    if !toolAllowed(ServerConfig{}, "list_directory") {
        t.Fatal("empty discovery allowlist should not block tools")
    }
}
