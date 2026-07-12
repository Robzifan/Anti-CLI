# E2E testing

`scripts/e2e.ps1` is a black-box PowerShell suite for a running mock gateway with memory and a configured read-only filesystem MCP server.

## Start the gateway

In one PowerShell window:

```powershell
cd C:\Users\anti\Desktop\notioncodex-gateway
$env:NOTION_CODEX_MOCK = "true"
$env:NOTION_CODEX_MEMORY_ENABLED = "true"
$env:NOTION_CODEX_MEMORY_DB = "C:\Users\anti\.notioncodex\memory.db"
$env:NOTION_CODEX_MCP_CONFIG = "$PWD\mcp.servers.json"
go run ./cmd/gateway
```

If an MCP API key is configured, set it in the E2E shell too.

## Run

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
```

With an API key:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\e2e.ps1 -ApiKey $env:NOTION_CODEX_MCP_API_KEY
```

## Assertions

1. health and mock model discovery;
2. MCP tool discovery;
3. read-only policy excludes write tools;
4. real MCP `list_directory` call;
5. stable local conversation continuity;
6. memory-enabled response path.

The script uses a unique conversation ID and does not mutate project files.

## Live validation

Live tests are intentionally separate because they require an authorized session and must not run in CI. See [Live validation](live-validation.md).
