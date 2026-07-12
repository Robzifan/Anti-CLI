# Architecture

## Runtime overview

```text
Responses-compatible client / Codex
  -> API server
     -> conversation + memory assembler
     -> optional gateway-owned MCP tool loop
     -> inference transport
        -> live Notion adapter or mock adapter
     -> SSE / JSON response

MCP calls
  -> policy / approval check
  -> stdio or Streamable HTTP MCP server
  -> tool execution record
  -> SQLite outbox
  -> memory worker
  -> searchable tool outcome
```

## Modules

| Module | Responsibility |
|---|---|
| `internal/api` | HTTP routes, Responses subset, SSE, MCP diagnostic/call/approval endpoints |
| `internal/config` | Environment configuration and validation |
| `internal/domain` | Transport-neutral models and inference events |
| `internal/models` | Discovery-fed model registry |
| `internal/notion` | Isolated live adapter, transcript codec, model picker, fixture research utilities |
| `internal/memory` | Conversations, summaries, memory retrieval, tool history, approvals, SQLite/WAL, outbox worker |
| `internal/mcp` | MCP config, stdio and Streamable HTTP discovery/call clients, policy filtering |
| `internal/telemetry` | Structured logging and redaction-aware diagnostics |

## Design constraints

- The Go gateway is a modular monolith. It is intentionally one local process while interfaces keep protocol, storage, MCP, and API concerns separable.
- Private Notion protocol details stay under `internal/notion`; API and memory packages must not depend on wire-format details.
- Memory is local-first. Upstream thread reuse is not a source of truth.
- MCP execution, memory, and policy are gateway concerns. The CLI/UI layer may manage configuration and prompts but must not bypass gateway policy.
- The default listener is loopback. Binding to a broader address requires explicit hardening, authentication, and TLS/reverse-proxy review.

## Account and project isolation

The current runtime uses a local user identity and explicit project identifiers supplied in request metadata. Memory queries are scoped by user and project. Conversation summaries and raw recent turns are conversation-local; project memory retrieval is intended to be reusable across chats in the same project.

Cross-account routing, user-global memory, and standalone CLI profiles are planned capabilities, not current production guarantees.

## Model and reasoning state

Model discovery can expose model picker metadata including supported reasoning efforts. This is capability metadata only. The current live inference builder does not yet have a fixture-verified upstream reasoning-effort request mapping. The product must label reasoning selection as unverified until that evidence exists.

## Lifecycle

1. Gateway loads config and starts mock or live transport.
2. Live mode discovers models from the authorized workspace.
3. A request resolves conversation/project scope and assembles bounded context.
4. The selected transport streams normalized events.
5. The API returns JSON or SSE.
6. Completed turns are persisted and enqueue derived memory work.
7. The worker creates summaries, memory records, FTS entries, and tool outcomes.

See [Memory and context](memory-context.md), [MCP](mcp.md), and [Protocol research](protocol-research.md).
