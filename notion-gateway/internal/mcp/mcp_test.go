package mcp

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestLoadAcceptsScopedStdioServer(t *testing.T) {
    root := t.TempDir()
    configPath := filepath.Join(root, "mcp.json")
    content, err := json.Marshal(Config{Servers: map[string]ServerConfig{
        "filesystem": {
            Transport: "stdio",
            Command: "npx.cmd",
            WorkingDirectory: root,
            AllowedTools: []string{"read_file"},
        },
    }})
    if err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(configPath, content, 0o600); err != nil {
        t.Fatal(err)
    }
    manager, err := Load(configPath)
    if err != nil {
        t.Fatal(err)
    }
    if len(manager.servers) != 1 {
        t.Fatalf("servers=%d", len(manager.servers))
    }
}

func TestLoadRejectsUnsupportedTransport(t *testing.T) {
    root := t.TempDir()
    configPath := filepath.Join(root, "mcp.json")
    content := `{"servers":{"remote":{"transport":"ftp","command":"unused"}}}`
    if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
        t.Fatal(err)
    }
    if _, err := Load(configPath); err == nil {
        t.Fatal("expected unsupported transport error")
    }
}
