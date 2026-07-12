# MCP integration

The gateway supports policy-filtered MCP discovery and execution.

## Supported transports

```text
stdio
streamable_http
```

`stdio` servers are launched as child processes. Streamable HTTP servers use MCP initialization, session headers, `tools/list`, and `tools/call`.

## Filesystem example

Copy the example config:

```powershell
Copy-Item .\mcp.servers.example.json .\mcp.servers.json
$env:NOTION_CODEX_MCP_CONFIG = "$PWD\mcp.servers.json"
```

A safe read-only configuration:

```json
{
  "servers": {
    "project-filesystem": {
      "transport": "stdio",
      "command": "npx.cmd",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "working_directory": "C:\\Users\\anti\\Desktop\\notioncodex-gateway",
      "allowed_tools": [
        "list_allowed_directories",
        "list_directory",
        "list_directory_with_sizes",
        "directory_tree",
        "get_file_info",
        "read_text_file",
        "read_multiple_files",
        "search_files"
      ],
      "approval_required_tools": []
    }
  }
}
```

Do not grant `C:\` or an entire user profile as the filesystem root.

## Discovery

```powershell
Invoke-WebRequest http://127.0.0.1:8787/v1/mcp/tools | Select-Object -Expand Content
```

## Direct local tool calls

```powershell
$body = @{ server = "project-filesystem"; name = "list_directory"; arguments = @{ path = "." }; conversation = "project-session"; metadata = @{ ncg_project_id = "notioncodex-gateway" } } | ConvertTo-Json -Depth 8
Invoke-WebRequest -Method POST -Uri "http://127.0.0.1:8787/v1/mcp/call" -ContentType "application/json" -Body $body | Select-Object -Expand Content
```

Direct calls are loopback-only. Every result is recorded through the tool-memory/outbox path when memory is enabled.

## Policy

`allowed_tools` is an allowlist. Keep it read-only until approval flow is tested.

`approval_required_tools` identifies tools that should create a pending approval rather than execute immediately. See [MCP approvals](mcp-approvals.md).

## Remote servers

Use `streamable_http` for remote MCP servers:

```json
{
  "transport": "streamable_http",
  "url": "https://mcp.example.com/mcp",
  "bearer_token_env": "REMOTE_MCP_TOKEN",
  "allowed_tools": ["read_only_tool"]
}
```

Use HTTPS outside loopback. Keep bearer tokens in environment variables, not JSON config.

## Current implementation note

MCP discovery/call is implemented. Persistent MCP sessions, OAuth management, CLI/TUI MCP management, and production-grade restart supervision are planned work.
