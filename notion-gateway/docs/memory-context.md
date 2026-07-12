# Memory and context

## Purpose

The memory subsystem keeps useful engineering context without replaying an entire chat history into every model request.

```text
current request
+ current conversation summary
+ recent turns
+ project-scoped retrieved memory
+ verified tool history
+ current MCP/web grounding when available
= bounded upstream context
```

## Implemented layers

| Layer | Scope | Use |
|---|---|---|
| Recent turns | Conversation | Short-term continuity |
| Rolling summary | Conversation | Compression of prior turns |
| Memory records | User + project | Decisions, bugs, constraints, open loops |
| Tool executions | User + project + conversation | Verified tool results, variants, failures |
| Tool outcomes | Project search | Compact evidence derived by worker |

## Conversation identity

The gateway resolves a conversation using, in priority order:

1. `previous_response_id`;
2. explicit `conversation`;
3. `metadata.ncg_conversation_id`;
4. `X-NotionCodex-Conversation-ID` header;
5. a new local conversation.

When memory is enabled the response includes `X-NotionCodex-Conversation-ID`.

## Cross-chat behavior

Recent turns and summaries are conversation-local. Project-scoped memory records are retrieved by exact user/project scope and can be reused by a later conversation for the same project. The system intentionally does not replay raw turns from unrelated chats.

User-global cross-project memory, explicit promotion/demotion, and rich memory management UX are planned work.

## Write path

On a completed assistant turn:

```text
assistant message
+ turn completion
+ memory.consolidate outbox event
= one SQLite transaction
```

A background worker then:

```text
extracts decisions/bugs/open loops
updates FTS records
builds a rolling summary
records durable tool outcomes
retries failed derived work with backoff
```

Partial/failed assistant output is not promoted to semantic memory.

## Retrieval and budgets

Context is bounded by `NOTION_CODEX_MEMORY_MAX_INJECTED_TOKENS`.

Priority order:

1. current user request;
2. current task/safety context;
3. rolling summary;
4. recent turns;
5. project memory;
6. verified tool history;
7. optional grounding.

Weak, redundant, stale, or cross-project records are excluded before prompt assembly.

## Safety

Before storage, FTS indexing, memory retrieval, or tool-memory persistence, text is scanned/redacted for credential-like values. Memory never intentionally stores raw session cookies, bearer tokens, passwords, or API keys.

Retrieved memory is framed as data, not privileged instructions. It must not override the current user request or gateway policy.

## Operations

Enable memory:

```powershell
$env:NOTION_CODEX_MEMORY_ENABLED = "true"
$env:NOTION_CODEX_MEMORY_DB = "C:\Users\anti\.notioncodex\memory.db"
```

SQLite parent directories are created automatically for normal filesystem paths.

## Current gaps

- No user-facing memory CLI/TUI yet.
- No embedding/vector retrieval yet; FTS is the baseline.
- No production retention/purge UI yet.
- Cross-project user memory promotion is not implemented.
- Memory quality depends on confirmed assistant/tool outcomes; it does not preserve hidden reasoning.
