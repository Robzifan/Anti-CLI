# Test plan

## Local baseline

```powershell
go mod tidy
gofmt -w .
go test ./...
go vet ./...
```

For race-sensitive work:

```powershell
go test -race ./...
```

## Unit and package tests

Required coverage includes:

- config validation;
- model registry behavior;
- Responses JSON and SSE ordering;
- Unicode preservation;
- latest structured user-message extraction;
- decoder terminal and patch-offset regressions;
- memory sanitization, summaries, outbox, approvals, tool execution scope;
- MCP config, stdio/HTTP transport, allowlists, and direct calls;
- API key protection for MCP endpoints.

## Integration tests

Use temporary SQLite databases and mock transports to verify:

```text
conversation continuity
restart persistence
project isolation
outbox retry
MCP tool execution memory
approval lifecycle
Codex tool-loop parser
```

## Black-box E2E

Run `scripts/e2e.ps1` against a running mock gateway with filesystem MCP. It verifies health, models, MCP discovery, policy-filtered tools, a real read-only tool call, and conversation continuity.

## Manually gated live tests

Live tests require an authorized account and must never run in CI:

```text
model discovery
non-stream inference
SSE streaming
memory continuity
MCP discovery/call
fixture-backed decoder regression
```

## Security tests

```text
secret scanner
fixture validator
credential redaction
cross-project memory leakage
cross-account routing once implemented
approval replay/expiry
MCP allowlist bypass attempts
```

## Release gate

A release candidate requires:

```text
gofmt clean
go test ./...
go vet ./...
go test -race ./...
secret scan
MCP E2E pass
manual live smoke pass
```
