package mcp

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestStreamableHTTPToolsAndCall(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var request struct {
            ID json.RawMessage `json:"id"`
            Method string `json:"method"`
        }
        if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
            t.Fatal(err)
        }
        w.Header().Set("Content-Type", "application/json")
        switch request.Method {
        case "initialize":
            w.Header().Set("Mcp-Session-Id", "test-session")
            _, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
        case "notifications/initialized":
            w.WriteHeader(http.StatusAccepted)
        case "tools/list":
            _, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"read_text_file","description":"read","inputSchema":{"type":"object"}},{"name":"write_file","description":"write","inputSchema":{"type":"object"}}]}}`))
        case "tools/call":
            _, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"ok"}],"isError":false}}`))
        default:
            t.Fatalf("unexpected method %q", request.Method)
        }
    }))
    defer server.Close()

    cfg := ServerConfig{Transport:"streamable_http", URL:server.URL, AllowedTools:[]string{"read_text_file"}}
    tools, err := listHTTP(context.Background(), "remote", cfg)
    if err != nil { t.Fatal(err) }
    if len(tools) != 1 || tools[0].Name != "read_text_file" { t.Fatalf("tools=%#v", tools) }

    result, err := callHTTP(context.Background(), "remote", cfg, "read_text_file", json.RawMessage(`{"path":"README.md"}`))
    if err != nil { t.Fatal(err) }
    if result.IsError || len(result.Content) == 0 { t.Fatalf("result=%#v", result) }
}
