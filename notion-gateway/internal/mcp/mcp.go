// Package mcp provides a deliberately small, policy-filtered MCP stdio client.
package mcp

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "net"
    "net/url"
    "path/filepath"
    "sort"
    "strings"
    "time"
)

type Config struct {
    Servers map[string]ServerConfig `json:"servers"`
}

type ServerConfig struct {
    Transport        string   `json:"transport"`
    Command          string   `json:"command"`
    Args             []string `json:"args"`
    WorkingDirectory string   `json:"working_directory"`
    AllowedTools     []string `json:"allowed_tools"`
    URL              string   `json:"url"`
    BearerTokenEnv   string   `json:"bearer_token_env"`
    ApprovalRequiredTools []string `json:"approval_required_tools"`
}

type Tool struct {
    Server      string          `json:"server"`
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type Manager struct {
    servers map[string]ServerConfig
}

func Load(path string) (*Manager, error) {
    raw, err := os.ReadFile(path)
    if err != nil { return nil, fmt.Errorf("read MCP config: %w", err) }
    var cfg Config
    dec := json.NewDecoder(bytes.NewReader(raw))
    dec.DisallowUnknownFields()
    if err := dec.Decode(&cfg); err != nil { return nil, fmt.Errorf("decode MCP config: %w", err) }
    if len(cfg.Servers) == 0 { return nil, errors.New("MCP config has no servers") }
    for name, server := range cfg.Servers {
        if strings.TrimSpace(name) == "" { return nil, errors.New("MCP server name is required") }
        switch server.Transport {
        case "stdio":
            if strings.TrimSpace(server.Command) == "" { return nil, fmt.Errorf("MCP server %q command is required", name) }
            if server.WorkingDirectory != "" { info, err := os.Stat(server.WorkingDirectory); if err != nil || !info.IsDir() { return nil, fmt.Errorf("MCP server %q working_directory is unavailable", name) } }
        case "streamable_http":
            if err := validateHTTPURL(server.URL); err != nil { return nil, fmt.Errorf("MCP server %q: %w", name, err) }
        default:
            return nil, fmt.Errorf("MCP server %q has unsupported transport %q", name, server.Transport)
        }
    }
    return &Manager{servers: cfg.Servers}, nil
}

// Tools discovers policy-allowed tool definitions. Each stdio discovery uses a
// short-lived process; execution sessions will be introduced with the response tool loop.
func (m *Manager) Tools(ctx context.Context) ([]Tool, error) {
    var out []Tool
    var failures []string
    for name, cfg := range m.servers {
        var tools []Tool
        var err error
        switch cfg.Transport {
        case "stdio":
            tools, err = listStdio(ctx, name, cfg)
        case "streamable_http":
            tools, err = listHTTP(ctx, name, cfg)
        }
        if err != nil {
            failures = append(failures, name+": "+err.Error())
            continue
        }
        out = append(out, tools...)
    }
    sort.Slice(out, func(i, j int) bool { if out[i].Server == out[j].Server { return out[i].Name < out[j].Name }; return out[i].Server < out[j].Server })
    if len(failures) > 0 { return out, errors.New(strings.Join(failures, "; ")) }
    return out, nil
}

type rpcResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id"`
    Result  json.RawMessage `json:"result"`
    Error   *struct { Message string `json:"message"` } `json:"error"`
}

func listStdio(ctx context.Context, name string, cfg ServerConfig) ([]Tool, error) {
    cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
    if cfg.WorkingDirectory != "" { cmd.Dir = filepath.Clean(cfg.WorkingDirectory) }
    stdin, err := cmd.StdinPipe(); if err != nil { return nil, err }
    stdout, err := cmd.StdoutPipe(); if err != nil { return nil, err }
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    if err := cmd.Start(); err != nil { return nil, fmt.Errorf("start: %w", err) }
    defer func() { _ = stdin.Close(); if cmd.Process != nil { _ = cmd.Process.Kill() }; _ = cmd.Wait() }()

    enc := json.NewEncoder(stdin)
    if err := enc.Encode(map[string]any{"jsonrpc":"2.0","id":1,"method":"initialize","params":map[string]any{"protocolVersion":"2025-03-26","capabilities":map[string]any{},"clientInfo":map[string]string{"name":"notioncodex-gateway","version":"0.1"}}}); err != nil { return nil, err }
    scanner := bufio.NewScanner(io.LimitReader(stdout, 2<<20))
    scanner.Buffer(make([]byte, 4096), 2<<20)
    init, err := nextResponse(scanner, "1")
    if err != nil { return nil, mcpErr(err, stderr.String()) }
    if init.Error != nil { return nil, fmt.Errorf("initialize: %s", init.Error.Message) }
    if err := enc.Encode(map[string]any{"jsonrpc":"2.0","method":"notifications/initialized"}); err != nil { return nil, err }
    if err := enc.Encode(map[string]any{"jsonrpc":"2.0","id":2,"method":"tools/list","params":map[string]any{}}); err != nil { return nil, err }
    listed, err := nextResponse(scanner, "2")
    if err != nil { return nil, mcpErr(err, stderr.String()) }
    if listed.Error != nil { return nil, fmt.Errorf("tools/list: %s", listed.Error.Message) }
    var result struct { Tools []struct { Name string `json:"name"`; Description string `json:"description"`; InputSchema json.RawMessage `json:"inputSchema"` } `json:"tools"` }
    if err := json.Unmarshal(listed.Result, &result); err != nil { return nil, fmt.Errorf("decode tools/list: %w", err) }
    allowed := map[string]struct{}{}
    for _, tool := range cfg.AllowedTools { allowed[tool] = struct{}{} }
    out := make([]Tool, 0, len(result.Tools))
    for _, item := range result.Tools {
        if len(allowed) > 0 { if _, ok := allowed[item.Name]; !ok { continue } }
        out = append(out, Tool{Server:name, Name:item.Name, Description:item.Description, InputSchema:item.InputSchema})
    }
    return out, nil
}

func nextResponse(scanner *bufio.Scanner, wanted string) (rpcResponse, error) {
    deadline := time.NewTimer(20 * time.Second); defer deadline.Stop()
    lines := make(chan string, 1)
    errCh := make(chan error, 1)
    go func() { if scanner.Scan() { lines <- scanner.Text(); return }; errCh <- scanner.Err() }()
    select {
    case <-deadline.C: return rpcResponse{}, errors.New("MCP response timeout")
    case err := <-errCh: if err == nil { err = io.EOF }; return rpcResponse{}, err
    case line := <-lines:
        var response rpcResponse
        if err := json.Unmarshal([]byte(line), &response); err != nil { return rpcResponse{}, err }
        if strings.Trim(string(response.ID), `"`) != wanted { return nextResponse(scanner, wanted) }
        return response, nil
    }
}

func mcpErr(err error, stderr string) error {
    stderr = strings.TrimSpace(stderr)
    if stderr == "" { return err }
    if len(stderr) > 500 { stderr = stderr[:500] + "…" }
    return fmt.Errorf("%w: %s", err, stderr)
}


type CallResult struct {
    Server  string          `json:"server"`
    Name    string          `json:"name"`
    Content json.RawMessage `json:"content"`
    IsError bool            `json:"is_error"`
}

// Call invokes one policy-allowed MCP tool through a fresh stdio session.
func (m *Manager) Call(ctx context.Context, serverName, toolName string, arguments json.RawMessage) (CallResult, error) {
    cfg, ok := m.servers[serverName]
    if !ok { return CallResult{}, fmt.Errorf("unknown MCP server %q", serverName) }
    if !toolAllowed(cfg, toolName) { return CallResult{}, fmt.Errorf("MCP tool %q is not allowed for server %q", toolName, serverName) }
    if len(arguments) == 0 { arguments = json.RawMessage(`{}`) }
    var valid any
    if err := json.Unmarshal(arguments, &valid); err != nil { return CallResult{}, fmt.Errorf("invalid MCP tool arguments: %w", err) }
    switch cfg.Transport {
    case "stdio":
        return callStdio(ctx, serverName, cfg, toolName, arguments)
    case "streamable_http":
        return callHTTP(ctx, serverName, cfg, toolName, arguments)
    }
    return CallResult{}, fmt.Errorf("unsupported MCP transport %q", cfg.Transport)
}

func (m *Manager) RequiresApproval(serverName, toolName string) bool {
    cfg, ok := m.servers[serverName]; if !ok { return false }
    for _, tool := range cfg.ApprovalRequiredTools { if tool == toolName { return true } }
    return false
}

func toolAllowed(cfg ServerConfig, toolName string) bool {
    if len(cfg.AllowedTools) == 0 { return true }
    for _, allowed := range cfg.AllowedTools { if allowed == toolName { return true } }
    return false
}

func callStdio(ctx context.Context, serverName string, cfg ServerConfig, toolName string, arguments json.RawMessage) (CallResult, error) {
    cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
    if cfg.WorkingDirectory != "" { cmd.Dir = filepath.Clean(cfg.WorkingDirectory) }
    stdin, err := cmd.StdinPipe(); if err != nil { return CallResult{}, err }
    stdout, err := cmd.StdoutPipe(); if err != nil { return CallResult{}, err }
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    if err := cmd.Start(); err != nil { return CallResult{}, fmt.Errorf("start: %w", err) }
    defer func() { _ = stdin.Close(); if cmd.Process != nil { _ = cmd.Process.Kill() }; _ = cmd.Wait() }()

    enc := json.NewEncoder(stdin)
    if err := enc.Encode(map[string]any{"jsonrpc":"2.0","id":1,"method":"initialize","params":map[string]any{"protocolVersion":"2025-03-26","capabilities":map[string]any{},"clientInfo":map[string]string{"name":"notioncodex-gateway","version":"0.1"}}}); err != nil { return CallResult{}, err }
    scanner := bufio.NewScanner(io.LimitReader(stdout, 4<<20))
    scanner.Buffer(make([]byte, 4096), 4<<20)
    init, err := nextResponse(scanner, "1")
    if err != nil { return CallResult{}, mcpErr(err, stderr.String()) }
    if init.Error != nil { return CallResult{}, fmt.Errorf("initialize: %s", init.Error.Message) }
    if err := enc.Encode(map[string]any{"jsonrpc":"2.0","method":"notifications/initialized"}); err != nil { return CallResult{}, err }
    var args any
    if err := json.Unmarshal(arguments, &args); err != nil { return CallResult{}, err }
    if err := enc.Encode(map[string]any{"jsonrpc":"2.0","id":2,"method":"tools/call","params":map[string]any{"name":toolName,"arguments":args}}); err != nil { return CallResult{}, err }
    response, err := nextResponse(scanner, "2")
    if err != nil { return CallResult{}, mcpErr(err, stderr.String()) }
    if response.Error != nil { return CallResult{}, fmt.Errorf("tools/call: %s", response.Error.Message) }
    var result struct { Content json.RawMessage `json:"content"`; IsError bool `json:"isError"` }
    if err := json.Unmarshal(response.Result, &result); err != nil { return CallResult{}, fmt.Errorf("decode tools/call: %w", err) }
    return CallResult{Server:serverName, Name:toolName, Content:result.Content, IsError:result.IsError}, nil
}

func validateHTTPURL(raw string) error {
    u, err := url.Parse(raw); if err != nil || u.Host == "" { return errors.New("invalid streamable_http URL") }
    if u.Scheme == "https" { return nil }
    if u.Scheme != "http" { return errors.New("streamable_http URL must use https or loopback http") }
    host := u.Hostname(); ip := net.ParseIP(host)
    if host == "localhost" || (ip != nil && ip.IsLoopback()) { return nil }
    return errors.New("streamable_http URL must use https outside loopback")
}
