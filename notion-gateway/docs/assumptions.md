# Current status, assumptions, and gaps

## Confirmed in the current implementation

- Authorized live model discovery has been exercised locally.
- The gateway exposes a practical Responses-compatible subset.
- Live streaming has fixture-backed regressions for observed patch stream layouts.
- Local conversation/project memory, SQLite/WAL, outbox work, MCP discovery/call, and black-box mock E2E checks are implemented.
- Codex can use the live gateway for text inference when configured as a compatible provider.

## Important non-claims

- The private Notion browser protocol is version-unstable and only mappings backed by redacted fixtures are trusted.
- Model picker availability does not prove every model or capability is live-inference verified.
- Reasoning-effort metadata is discovered but selected reasoning effort is not yet fixture-verified as an upstream inference parameter.
- The gateway-owned Codex/MCP marker loop is experimental and is not a native Notion tool-call mapping.
- MCP write/destructive actions require a tested approval policy; read-only policy is the recommended default.

## Current gaps

- No standalone NotionCodex CLI fork yet; current `cmd/notion-codex` is a fixture/protocol utility.
- No profile/config.toml UX, multi-account router, quota dashboard, web research engine, or deep research UI.
- No embedding/vector retrieval or Postgres backend.
- No persistent MCP process supervisor or OAuth UI.
- No user-facing memory management UI or retention/purge workflow.

## Decision discipline

Do not add private protocol request fields, model capabilities, reasoning controls, or tool mappings based on names alone. Add them only with redacted fixture evidence, regression tests, and explicit documentation of confidence.
