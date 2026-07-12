# Configuration

The gateway reads environment variables at process start. Open a new terminal after using `setx`; persistent Windows environment variables do not update existing processes.

## Core variables

| Variable | Default | Description |
|---|---|---|
| `NOTION_CODEX_ADDRESS` | `127.0.0.1:8787` | Listener address. Keep loopback unless authentication/TLS is configured. |
| `NOTION_CODEX_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `NOTION_CODEX_MOCK` | `true` | Uses the synthetic `mock/echo` transport. |
| `NOTION_CODEX_LIVE` | `false` | Enables authorized live Notion transport. |
| `NOTION_SPACE_ID` | empty | Required in live mode. |
| `NOTION_TIMEZONE` | `UTC` | Time zone passed to the observed transcript context. |
| `NOTION_CODEX_REQUEST_TIMEOUT` | implementation default | Request timeout is currently compiled into the gateway configuration. |

## Live credential

Live mode requires:

```powershell
$env:NOTION_SESSION_COOKIE = "token_v2=<session-value>"
```

The gateway expects the complete cookie pair, not only the token value.

Do not commit this value, add it to project config, paste it into chat, or enable content debug logging while it is present. Session credentials may expire and should be supplied only to the shell that starts the gateway.

## Memory

| Variable | Default | Description |
|---|---|---|
| `NOTION_CODEX_MEMORY_ENABLED` | `false` | Enables conversation, summary, FTS, tool, and approval storage. |
| `NOTION_CODEX_MEMORY_DB` | `notioncodex-memory.db` | SQLite database path. Parent directories are created for filesystem paths. |
| `NOTION_CODEX_MEMORY_MAX_INJECTED_TOKENS` | `12000` | Hard cap for gateway-injected context. |
| `NOTION_CODEX_MEMORY_RECENT_TURNS` | `6` | Recent turns considered for continuity. |

Recommended Windows path:

```powershell
$env:NOTION_CODEX_MEMORY_DB = "C:\Users\anti\.notioncodex\memory.db"
```

## MCP

| Variable | Description |
|---|---|
| `NOTION_CODEX_MCP_CONFIG` | Path to MCP server JSON configuration. |
| `NOTION_CODEX_MCP_API_KEY` | Optional API key required for MCP endpoints when configured. |

Example read-only filesystem server:

```json
{
  "servers": {
    "project-filesystem": {
      "transport": "stdio",
      "command": "npx.cmd",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "working_directory": "C:\\Users\\anti\\Desktop\\notioncodex-gateway",
      "allowed_tools": ["list_directory", "read_text_file", "search_files"],
      "approval_required_tools": []
    }
  }
}
```

Remote MCP servers use `streamable_http` and should use HTTPS outside loopback. Use `bearer_token_env` rather than placing bearer tokens in JSON.

## API key usage

When `NOTION_CODEX_MCP_API_KEY` is set, call MCP endpoints with either:

```text
X-NotionCodex-API-Key: <key>
```

or:

```text
Authorization: Bearer <key>
```

## Current configuration model

The gateway currently uses environment variables and MCP JSON. A future standalone CLI will add profile-oriented TOML configuration. That CLI configuration is planned work; do not assume it exists in the current gateway.
