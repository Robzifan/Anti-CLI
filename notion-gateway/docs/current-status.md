# Current status

## Verified locally by the project operator

```text
Live model discovery
Live non-stream inference
Live SSE streaming
Fixture-backed stream decoder regressions
Conversation continuity
Project-scoped memory retrieval
MCP stdio discovery/call
MCP Streamable HTTP unit coverage
Read-only filesystem MCP E2E
Codex CLI text inference through gateway
```

## Implemented but still maturing

```text
Gateway-owned Codex/MCP compatibility loop
MCP approval persistence/API
HTTP MCP transport
Cross-chat project memory behavior
Tool execution memory
```

These require continued integration and live regression coverage before being treated as stable product contracts.

## Explicitly unverified or planned

```text
Upstream reasoning-effort request mapping
Standalone NotionCodex CLI fork
Multi-account routing
Quota/limit dashboard
Web search provider
Deep research workflow
Persistent MCP session supervisor
GitHub MCP integration
Full approval UX
Write/destructive tool workflows
```

## Recommended operational mode today

```text
Loopback gateway
Read-only MCP tools
Memory enabled
Manual live validation after protocol changes
No secrets in project files or logs
```
