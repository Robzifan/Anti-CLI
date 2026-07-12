# NotionCodex Gateway

A local, self-hosted Go gateway that lets an OpenAI Responses-compatible client use models available to an authorized Notion workspace.

> **Security note:** this project uses an authorized local Notion session. Never commit or share `token_v2`, cookies, API keys, raw HAR files, raw upstream payloads, private prompts, or account/workspace identifiers.

## What works today

- Live model discovery from an authorized Notion workspace.
- OpenAI-compatible `GET /v1/models` and `POST /v1/responses`.
- Streaming SSE and non-streaming responses.
- Fixture-backed live stream decoding for observed `patch-start`, `patch`, and `patch-sync` layouts.
- Local-first conversation continuity and project-scoped memory.
- SQLite/WAL persistence, rolling summaries, FTS retrieval, outbox worker, and tool execution memory.
- MCP `stdio` and Streamable HTTP discovery plus policy-filtered `tools/call`.
- Loopback-only direct MCP calls and optional API-key protection.
- A gateway-owned experimental Codex/MCP compatibility loop for Responses clients that provide tools.
- Windows E2E PowerShell test harness.

## Important limitations

- This is a local, authorized-use gateway; it does not bypass Notion access controls, billing, MFA, quotas, or account restrictions.
- Reasoning-effort metadata may be discovered, but selected reasoning effort is **not yet upstream-verified** and must not be treated as effective until a fixture-backed mapping exists.
- The Codex/MCP compatibility loop is experimental. It uses a gateway-owned marker protocol and is not a claim of compatibility with private Notion tool-call fields.
- MCP write/destructive tools must remain approval-gated. The included filesystem example is read-only.
- A standalone `notioncodex` CLI fork is planned; `cmd/notion-codex` currently provides fixture/protocol utilities.

## Quick start: mock mode

Requires Go 1.26.5 and Node.js only when using the filesystem MCP example.

```powershell
cd C:\Users\anti\Desktop\notioncodex-gateway
go mod tidy
go test ./...
go vet ./...
$env:NOTION_CODEX_MOCK = "true"
go run ./cmd/gateway
```

In a second shell:

```powershell
Invoke-WebRequest http://127.0.0.1:8787/healthz | Select-Object -Expand Content
```

## Quick start: live mode

Set non-secret settings permanently if desired, but provide the session credential only in the current shell.

```powershell
$env:NOTION_CODEX_LIVE = "true"
$env:NOTION_CODEX_MOCK = "false"
$env:NOTION_SPACE_ID = "<your-space-id>"
$env:NOTION_SESSION_COOKIE = "token_v2=<session-value>"
$env:NOTION_CODEX_MEMORY_ENABLED = "true"
$env:NOTION_CODEX_MEMORY_DB = "C:\Users\anti\.notioncodex\memory.db"
go run ./cmd/gateway
```

Validate models:

```powershell
Invoke-WebRequest http://127.0.0.1:8787/v1/models | Select-Object -Expand Content
```

See [Live validation](docs/live-validation.md) for streaming and memory checks.

## MCP quick start

Copy the included example to a local working config:

```powershell
Copy-Item .\mcp.servers.example.json .\mcp.servers.json
$env:NOTION_CODEX_MCP_CONFIG = "$PWD\mcp.servers.json"
```

The example starts the official filesystem MCP server with the repository as its only allowed root. It intentionally exposes read-only tools only.

Start the gateway, then discover tools:

```powershell
Invoke-WebRequest http://127.0.0.1:8787/v1/mcp/tools | Select-Object -Expand Content
```

Run the repository E2E suite against a running mock gateway:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
```

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [API reference](docs/api.md)
- [Memory and context](docs/memory-context.md)
- [MCP](docs/mcp.md)
- [MCP approvals](docs/mcp-approvals.md)
- [Codex MCP compatibility loop](docs/codex-tool-loop.md)
- [E2E testing](docs/e2e-testing.md)
- [Live validation](docs/live-validation.md)
- [Protocol research safety](docs/protocol-research.md)
- [Threat model](docs/threat-model.md)
- [Test plan](docs/test-plan.md)

## Project architecture

```text
Codex / compatible client
  -> NotionCodex Gateway (Go)
     -> memory + SQLite/WAL + outbox
     -> MCP stdio / HTTP manager
     -> live Notion transport
  -> authorized Notion workspace
```

The modular monolith and protocol boundary are documented in `docs/adr/`.

## License

Apache-2.0. See [LICENSE](LICENSE).
