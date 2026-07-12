# Live validation

Live validation uses an authorized local Notion account. Do not paste session cookies, raw HAR exports, raw response bodies, private prompts, emails, or workspace identifiers into issues, chat, or source control.

## Start live gateway

```powershell
$env:NOTION_CODEX_LIVE = "true"
$env:NOTION_CODEX_MOCK = "false"
$env:NOTION_SPACE_ID = "<space-id>"
$env:NOTION_SESSION_COOKIE = "token_v2=<session-value>"
$env:NOTION_CODEX_MEMORY_ENABLED = "true"
$env:NOTION_CODEX_MEMORY_DB = "C:\Users\anti\.notioncodex\memory.db"
go run ./cmd/gateway
```

## Smoke checks

```powershell
Invoke-WebRequest http://127.0.0.1:8787/v1/models | Select-Object -Expand Content
```

Send a short non-stream request using `notion-auto` and verify that a complete response is returned.

## Streaming regression

Use a fresh conversation ID and a deterministic prompt such as:

```text
Reply with exactly: streaming live works.
```

The SSE stream must deliver complete concatenated text, not an early fragment. The repository includes fixture-backed regressions for observed `patch-start` and `patch-sync` offsets.

## Memory regression

1. Send a short project decision with a new `ncg_conversation_id`.
2. Send a second request with the same conversation/project identifiers.
3. Verify that the model recalls the decision without duplicated text or truncated output.

## Sanitized fixture workflow

When live decoding differs from tests, manually export a response body from browser DevTools, never a HAR or request headers. Use `scripts/sanitize_live_frames.py` locally to create a redacted fixture, review it manually, and retain only the minimum frames needed for regression coverage.

The sanitizer must not be treated as proof that a fixture is safe; manual review is required.
