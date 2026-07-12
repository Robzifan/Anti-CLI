# Safe Notion protocol research

The Notion browser protocol is private and may change without notice. The gateway accepts mappings only when supported by authorized, manually reviewed, redacted fixtures.

## Hard safety boundary

Use only accounts and workspaces you are authorized to access.

Never capture, commit, upload, or log:

```text
token_v2
cookies
Authorization headers
auth.json
API keys
private prompts
emails
stable user/workspace/page IDs
raw HAR files
hidden chain-of-thought
```

Do not attempt to bypass MFA, CAPTCHA, billing, quotas, suspensions, or server controls.

## Fixture workflow

1. Trigger one small, safe inference or model-discovery request in an authorized browser session.
2. Copy only the response body, not request headers or a HAR file.
3. Keep the raw file local and outside the repository.
4. Run `scripts/sanitize_live_frames.py` locally.
5. Manually review the redacted output.
6. Keep only the minimum frames needed for a decoder/request-mapping regression.
7. Validate the fixture envelope:

```powershell
go run ./cmd/notion-codex probe-fixture .\fixture.redacted.json
```

The sanitizer is an aid, not proof that output is safe. It may miss private values or preserve structural details that should be reviewed before sharing.

## Evidence rule

A protocol field may be implemented only when all conditions hold:

```text
- observed in an authorized response/request;
- sanitized fixture retained;
- parser/request mapping regression test added;
- behavior documented as observed, not assumed;
- no credentials or private content preserved.
```

## Live decoder fixtures

Observed live streams include patch lifecycle frames such as:

```text
patch-start
patch
patch-sync
record-map
```

The decoder preserves stream offsets established by `patch-start` and `patch-sync`; synthetic redacted regressions live under `internal/notion/inference/transcript`.

## Normalized stream harness

`decode-normalized-stream` reads a project-owned NDJSON format for downstream behavior tests:

```json
{"kind":"delta","text":"<redacted-text>"}
{"kind":"done"}
```

It is not a description of the upstream wire protocol.
