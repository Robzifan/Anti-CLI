> Historical ADR. It records an accepted design decision at the time it was made. See [Current status](../current-status.md) for implemented capability.

# ADR 0003: Observed model-picker discovery adapter

- Status: Accepted
- Date: 2026-07-11

## Observation

An authorized browser session observed a successful JSON request on 2026-07-11:

- Method: `POST`
- Path: `/api/v3/getAvailableModels`
- Body shape: `{"spaceId":"<workspace-id>"}`
- Response roots: `restrictedAccessModelsInPickerConfig` and `models`

No cookies, authorization values, raw headers, or private user content are retained in fixtures.

## Decision

All knowledge of this contract is isolated in `internal/notion/discovery/picker`. The adapter builds the observed request through a session interface and converts every returned model dynamically. It contains no list of model IDs.

The parser records upstream ID, display name, provider-family hint, picker availability, reasoning-effort metadata, route presence, restricted-access codename, and an optional approaching-rate-limit flag. UI card scores are intentionally ignored. Vision, web search, tool use, reasoning summaries, context limits, price, and quota are not inferred.

Public IDs are deterministically derived as `notion/<upstream-id>`. `isDisabled=false` means available in the picker snapshot, not proven inference success. A later probe must verify inference separately.

## Consequences

The endpoint may change because it is private and version-unstable. Parser failures must surface as protocol diagnostics and trigger rediscovery rather than fallback to a static model list. Authentication remains behind the session abstraction and is not implemented by this ADR.
