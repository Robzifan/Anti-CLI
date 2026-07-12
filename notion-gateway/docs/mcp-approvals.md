# MCP approvals and API key

## API key

Set an optional local MCP API key:

```powershell
$env:NOTION_CODEX_MCP_API_KEY = "<long-random-value>"
```

When configured, these endpoints require either `X-NotionCodex-API-Key` or `Authorization: Bearer <key>`:

```text
GET  /v1/mcp/tools
POST /v1/mcp/call
POST /v1/mcp/approvals
```

The key must not be committed or logged.

## Approval-required tools

Configure write/destructive tools explicitly:

```json
{
  "allowed_tools": ["read_text_file", "write_file"],
  "approval_required_tools": ["write_file"]
}
```

A call to an approval-required tool returns HTTP 202 with a pending approval record instead of executing.

## Resolve an approval

```powershell
$body = @{ id = "<approval-id>"; action = "approved" } | ConvertTo-Json
Invoke-WebRequest -Method POST -Uri "http://127.0.0.1:8787/v1/mcp/approvals" -ContentType "application/json" -Headers @{ "X-NotionCodex-API-Key" = $env:NOTION_CODEX_MCP_API_KEY } -Body $body
```

Use `denied` to reject.

Approvals are:

```text
pending
approved
denied
expired
```

They expire after a bounded period and cannot be resolved twice. Tool arguments containing credential-like values are rejected before approval persistence.

## Current gap

The current API persists approval state. Full interactive approval UX, diff previews, permission profiles, and CLI approval commands are planned for the standalone NotionCodex CLI.
