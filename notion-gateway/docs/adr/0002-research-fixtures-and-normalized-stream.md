> Historical ADR. It records an accepted design decision at the time it was made. See [Current status](../current-status.md) for implemented capability.

# ADR 0002: Redacted research fixtures and normalized stream

- Status: Accepted
- Date: 2026-07-11

## Context

No authorized, redacted capture of the private Notion AI protocol is available. Implementing an endpoint, payload mapper, model identifier, or status-code mapping without evidence would create a false contract and a security risk.

## Decision

The research harness has two deliberately separate formats:

1. A strict JSON fixture envelope whose `payload` remains opaque. It records a sanitized observation without claiming that any payload field is stable.
2. A project-owned normalized NDJSON stream containing only `delta`, `done`, or categorized `error` frames. It exercises cancellation, size limits, terminal-state handling, and downstream event flow without representing an upstream wire format.

Fixture loading is read-only, limited to 1 MiB, rejects trailing JSON documents, rejects unknown envelope fields, and recursively rejects credential-shaped keys and bearer/JWT-shaped values. CLI inputs must be regular files.

Error categories are assigned only by a future concrete adapter backed by observed fixtures. The protocol package does not infer private semantics from HTTP status codes or unknown payload fields.

## Consequences

- Parser and state behavior can be tested safely before a live adapter exists.
- Raw captures and credentials remain outside the repository.
- A future adapter must provide a fixture-backed mapping into normalized frames.
- The normalized format must never be documented as the Notion wire protocol.
