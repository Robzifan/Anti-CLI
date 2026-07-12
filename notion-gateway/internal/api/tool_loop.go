package api

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "regexp"
    "strings"

    "github.com/notioncodex/gateway/internal/domain"
    "github.com/notioncodex/gateway/internal/memory"
    "github.com/notioncodex/gateway/internal/ports"
)

var gatewayToolCallPattern = regexp.MustCompile(`(?s)<gateway_mcp_call>\s*(\{.*?\})\s*</gateway_mcp_call>`)

type gatewayToolCall struct {
    Server string `json:"server"`
    Name string `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}

// runGatewayToolLoop is a gateway-owned MCP compatibility loop for Responses
// clients that supply tools. Approval-required tools are never auto-executed.
func (s *Server) runGatewayToolLoop(ctx context.Context, model, baseInput string, turn memory.Turn) (string, error) {
    if s.mcp == nil { return "", errors.New("MCP is not configured") }
    tools, err := s.mcp.Tools(ctx)
    if err != nil { return "", err }
    prompt := baseInput + "\n\n<gateway_mcp_tools>\n"
    for _, tool := range tools { prompt += fmt.Sprintf("- server=%s name=%s description=%s\n", tool.Server, tool.Name, tool.Description) }
    prompt += "</gateway_mcp_tools>\nIf a tool is needed, output only <gateway_mcp_call>{\"server\":\"...\",\"name\":\"...\",\"arguments\":{}}</gateway_mcp_call>. Otherwise answer normally.\n"

    for attempt := 0; attempt < 3; attempt++ {
        text, err := s.collectInference(ctx, model, prompt, turn)
        if err != nil { return "", err }
        call, ok := parseGatewayToolCall(text)
        if !ok { return text, nil }
        if s.mcp.RequiresApproval(call.Server, call.Name) { return "", fmt.Errorf("tool %s.%s requires explicit approval", call.Server, call.Name) }
        result, err := s.mcp.Call(ctx, call.Server, call.Name, call.Arguments)
        if err != nil { return "", err }
        if s.memory != nil {
            status := "succeeded"; if result.IsError { status = "failed" }
            _, _ = s.memory.RecordToolExecution(ctx, memory.ToolExecution{Scope:turn.Scope, ConversationID:turn.ConversationID, TurnID:turn.ID, ToolName:result.Server+"."+result.Name, Operation:"tools/call", Status:status, OutputSummary:truncateMCPContent(result.Content), IdempotencyKey:"loop-"+turn.ID+fmt.Sprintf("-%d", attempt)})
        }
        prompt = fmt.Sprintf("%s\n\n<gateway_mcp_tool_result server=%q name=%q>\n%s\n</gateway_mcp_tool_result>\nAnswer the user using this result.\n", baseInput, result.Server, result.Name, truncateMCPContent(result.Content))
    }
    return "", errors.New("MCP tool loop exceeded maximum iterations")
}

func (s *Server) collectInference(ctx context.Context, model, input string, turn memory.Turn) (string, error) {
    stream, err := s.transport.Infer(ctx, domain.InferenceRequest{Model:model, Input:input})
    if err != nil { return "", err }
    defer stream.Close()
    var out strings.Builder
    for {
        event, err := stream.Recv(ctx)
        if errors.Is(err, io.EOF) { return out.String(), nil }
        if err != nil { return "", err }
        switch event.Kind {
        case domain.EventDelta:
            out.WriteString(event.Text)
        case domain.EventTool:
            s.recordToolEvent(ctx, turn, event.Tool, "gateway-tool-loop")
        case domain.EventError:
            return "", event.Err
        case domain.EventDone:
            return out.String(), nil
        }
    }
}

func parseGatewayToolCall(text string) (gatewayToolCall, bool) {
    match := gatewayToolCallPattern.FindStringSubmatch(text)
    if len(match) != 2 { return gatewayToolCall{}, false }
    var call gatewayToolCall
    if err := json.Unmarshal([]byte(match[1]), &call); err != nil || call.Server == "" || call.Name == "" { return gatewayToolCall{}, false }
    if len(call.Arguments) == 0 { call.Arguments = json.RawMessage(`{}`) }
    return call, true
}

type staticEventStream struct { events []domain.InferenceEvent; index int }
func (s *staticEventStream) Recv(context.Context) (domain.InferenceEvent, error) { if s.index >= len(s.events) { return domain.InferenceEvent{}, io.EOF }; event:=s.events[s.index]; s.index++; return event,nil }
func (s *staticEventStream) Close() error { return nil }
var _ ports.EventStream = (*staticEventStream)(nil)
