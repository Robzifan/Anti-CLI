package mcp

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
)

type httpSession struct {
    url       string
    token     string
    sessionID string
    client    *http.Client
}

func listHTTP(ctx context.Context, name string, cfg ServerConfig) ([]Tool, error) {
    session, err := newHTTPSession(ctx, cfg)
    if err != nil { return nil, err }
    response, err := session.rpc(ctx, 2, "tools/list", map[string]any{})
    if err != nil { return nil, err }
    if response.Error != nil { return nil, fmt.Errorf("tools/list: %s", response.Error.Message) }
    var result struct {
        Tools []struct {
            Name string `json:"name"`
            Description string `json:"description"`
            InputSchema json.RawMessage `json:"inputSchema"`
        } `json:"tools"`
    }
    if err := json.Unmarshal(response.Result, &result); err != nil { return nil, fmt.Errorf("decode tools/list: %w", err) }
    tools := make([]Tool, 0, len(result.Tools))
    for _, item := range result.Tools {
        if !toolAllowed(cfg, item.Name) { continue }
        tools = append(tools, Tool{Server:name, Name:item.Name, Description:item.Description, InputSchema:item.InputSchema})
    }
    return tools, nil
}

func callHTTP(ctx context.Context, serverName string, cfg ServerConfig, toolName string, arguments json.RawMessage) (CallResult, error) {
    session, err := newHTTPSession(ctx, cfg)
    if err != nil { return CallResult{}, err }
    var args any
    if err := json.Unmarshal(arguments, &args); err != nil { return CallResult{}, err }
    response, err := session.rpc(ctx, 2, "tools/call", map[string]any{"name":toolName, "arguments":args})
    if err != nil { return CallResult{}, err }
    if response.Error != nil { return CallResult{}, fmt.Errorf("tools/call: %s", response.Error.Message) }
    var result struct { Content json.RawMessage `json:"content"`; IsError bool `json:"isError"` }
    if err := json.Unmarshal(response.Result, &result); err != nil { return CallResult{}, fmt.Errorf("decode tools/call: %w", err) }
    return CallResult{Server:serverName, Name:toolName, Content:result.Content, IsError:result.IsError}, nil
}

func newHTTPSession(ctx context.Context, cfg ServerConfig) (*httpSession, error) {
    token := ""
    if cfg.BearerTokenEnv != "" {
        token = os.Getenv(cfg.BearerTokenEnv)
        if token == "" { return nil, fmt.Errorf("MCP bearer token environment %q is empty", cfg.BearerTokenEnv) }
    }
    session := &httpSession{url:cfg.URL, token:token, client:&http.Client{Timeout:25*time.Second}}
    response, err := session.rpc(ctx, 1, "initialize", map[string]any{"protocolVersion":"2025-03-26", "capabilities":map[string]any{}, "clientInfo":map[string]string{"name":"notioncodex-gateway", "version":"0.1"}})
    if err != nil { return nil, err }
    if response.Error != nil { return nil, fmt.Errorf("initialize: %s", response.Error.Message) }
    if err := session.notify(ctx, "notifications/initialized", map[string]any{}); err != nil { return nil, err }
    return session, nil
}

func (s *httpSession) notify(ctx context.Context, method string, params any) error {
    _, err := s.request(ctx, map[string]any{"jsonrpc":"2.0", "method":method, "params":params})
    return err
}

func (s *httpSession) rpc(ctx context.Context, id int, method string, params any) (rpcResponse, error) {
    body, err := s.request(ctx, map[string]any{"jsonrpc":"2.0", "id":id, "method":method, "params":params})
    if err != nil { return rpcResponse{}, err }
    return decodeHTTPResponse(body)
}

func (s *httpSession) request(ctx context.Context, payload any) ([]byte, error) {
    raw, err := json.Marshal(payload)
    if err != nil { return nil, err }
    request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(raw))
    if err != nil { return nil, err }
    request.Header.Set("Content-Type", "application/json")
    request.Header.Set("Accept", "application/json, text/event-stream")
    if s.token != "" { request.Header.Set("Authorization", "Bearer "+s.token) }
    if s.sessionID != "" { request.Header.Set("Mcp-Session-Id", s.sessionID) }
    response, err := s.client.Do(request)
    if err != nil { return nil, err }
    defer response.Body.Close()
    if response.StatusCode < 200 || response.StatusCode >= 300 { return nil, fmt.Errorf("MCP HTTP status %s", response.Status) }
    if id := response.Header.Get("Mcp-Session-Id"); id != "" { s.sessionID = id }
    return io.ReadAll(io.LimitReader(response.Body, 2<<20))
}

func decodeHTTPResponse(body []byte) (rpcResponse, error) {
    body = bytes.TrimSpace(body)
    if bytes.HasPrefix(body, []byte("data:")) {
        scanner := bufio.NewScanner(bytes.NewReader(body))
        for scanner.Scan() {
            line := strings.TrimSpace(scanner.Text())
            if strings.HasPrefix(line, "data:") {
                body = bytes.TrimSpace([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))))
                break
            }
        }
        if err := scanner.Err(); err != nil { return rpcResponse{}, err }
    }
    var response rpcResponse
    if err := json.Unmarshal(body, &response); err != nil { return rpcResponse{}, fmt.Errorf("decode MCP HTTP response: %w", err) }
    return response, nil
}
