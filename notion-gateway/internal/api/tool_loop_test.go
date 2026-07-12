package api

import (
    "encoding/json"
    "testing"
)

func TestParseGatewayToolCall(t *testing.T) {
    text := ` <gateway_mcp_call>{"server":"project-filesystem","name":"read_text_file","arguments":{"path":"README.md"}}</gateway_mcp_call> `
    call, ok := parseGatewayToolCall(text)
    if !ok { t.Fatal("expected tool call") }
    if call.Server != "project-filesystem" || call.Name != "read_text_file" { t.Fatalf("call=%#v", call) }
    var args map[string]string
    if err := json.Unmarshal(call.Arguments, &args); err != nil { t.Fatal(err) }
    if args["path"] != "README.md" { t.Fatalf("args=%#v", args) }
}

func TestParseGatewayToolCallRejectsPlainText(t *testing.T) {
    if _, ok := parseGatewayToolCall("normal response"); ok { t.Fatal("unexpected tool call") }
}
