# API reference

The gateway implements a practical subset of the OpenAI Responses API.

## Health and discovery

```text
GET /healthz
GET /readyz
GET /v1/models
```

`/readyz` is successful when the model registry has at least one discovered model.

## Responses

```text
POST /v1/responses
```

Supported request fields:

```json
{
  "model": "notion-auto",
  "input": "Reply with exactly: hello.",
  "stream": false,
  "previous_response_id": "resp_...",
  "conversation": "optional-local-conversation-id",
  "metadata": {
    "ncg_conversation_id": "stable-client-key",
    "ncg_project_id": "project-name-or-id"
  },
  "tools": []
}
```

`input` may be a string or supported Responses-style text content. When structured conversation items are supplied, the gateway selects the latest `role: user` message before creating the upstream request. This prevents prior assistant text from being treated as the new user request.

### Streaming

Set `stream: true` to receive SSE events:

```text
response.created
response.output_item.added
response.content_part.added
response.output_text.delta
response.output_text.done
response.output_item.done
response.completed
```

### Conversation continuity

When memory is enabled the gateway returns:

```text
X-NotionCodex-Conversation-ID
```

Continuity can be selected with, in priority order:

1. `previous_response_id`;
2. `conversation`;
3. `metadata.ncg_conversation_id`;
4. `X-NotionCodex-Conversation-ID` request header.

## MCP endpoints

These endpoints are intended for local/loopback operation. If `NOTION_CODEX_MCP_API_KEY` is configured, provide the MCP API key.

```text
GET  /v1/mcp/tools
POST /v1/mcp/call
POST /v1/mcp/approvals
```

### List tools

```powershell
Invoke-WebRequest http://127.0.0.1:8787/v1/mcp/tools | Select-Object -Expand Content
```

### Call a tool

```json
{
  "server": "project-filesystem",
  "name": "list_directory",
  "arguments": { "path": "." },
  "conversation": "project-session",
  "metadata": { "ncg_project_id": "notioncodex-gateway" }
}
```

`POST /v1/mcp/call` is loopback-only. A policy-allowed read-only tool executes immediately. A tool configured in `approval_required_tools` returns HTTP 202 and a pending approval record instead.

### Resolve an approval

```json
{
  "id": "approval-id",
  "action": "approved"
}
```

Actions are `approved` or `denied`. Approvals expire and are one-time only.

## Gateway-owned tool loop

When a Responses client sends a non-empty `tools` field and MCP is configured, the gateway can advertise policy-filtered MCP tools to the upstream model through an experimental gateway-owned marker protocol. See [Codex tool loop](codex-tool-loop.md).

This is not a claim that private Notion tool-call fields are supported or mapped.

## Errors

Errors use an OpenAI-style envelope:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error",
    "request_id": "..."
  }
}
```

Common types include:

```text
invalid_request_error
upstream_error
mcp_error
mcp_tool_loop_error
authentication_error
approval_error
```
