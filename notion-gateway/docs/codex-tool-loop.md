# Codex MCP compatibility loop

## Purpose

The gateway includes an experimental compatibility loop for Responses clients such as Codex when the client supplies a non-empty `tools` array and MCP is configured.

## Flow

```text
Responses request with tools
-> gateway adds policy-filtered MCP tool catalog
-> upstream model may emit a gateway marker
-> gateway validates MCP policy
-> gateway runs read-only MCP call
-> gateway records tool execution
-> gateway reinfers with tool result
-> final answer returns to client
```

Marker format:

```text
<gateway_mcp_call>{"server":"project-filesystem","name":"read_text_file","arguments":{"path":"README.md"}}</gateway_mcp_call>
```

The loop is bounded to three iterations.

## Safety

- Tools outside the MCP allowlist are rejected.
- Approval-required tools are never auto-executed by this loop.
- Tool results are size-limited before continuation and are recorded in tool memory when enabled.
- The loop is gateway-owned. It does not infer private Notion tool-call fields and does not claim native upstream tool protocol compatibility.

## Current status

The protocol parser and latest-user-message handling are implemented. End-to-end behavior depends on the upstream model choosing the documented marker protocol. Treat this as experimental until dedicated mock and live fixture tests prove the full loop for a specific model.
