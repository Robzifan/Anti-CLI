> Historical ADR. It records an accepted design decision at the time it was made. See [Current status](../current-status.md) for implemented capability.

# ADR 0001: Modular monolith and strict upstream protocol boundary

- Status: Accepted
- Date: 2026-07-11

## Context

The upstream Notion AI browser protocol is private, changeable, and not yet confirmed by redacted captures. The gateway also needs stable OpenAI-facing APIs, model discovery, Codex import, memory, and storage.

## Decision

Use one deployable Go process with internal modules and explicit ports. `InferenceTransport` and `ModelDiscoverer` are the only inference/discovery dependencies of application code. All confirmed browser-protocol knowledge will live under `internal/notion/protocol` and concrete adapters under `internal/notion/transport`. Domain and HTTP packages must not import protocol packages.

The model registry is discovery-fed; an empty registry is valid but not ready. The mock transport advertises only a clearly synthetic `mock/echo` model. It is test/development data, never an assertion about Notion.

Codex import and memory/context engines remain separate modules because imported history is immutable source material, while memory is derived, searchable data with different retention rules. Storage implementations will satisfy ports rather than leak SQL into domain logic.

## Consequences

- Private protocol changes are isolated.
- Mock and fixture-driven compatibility tests do not require credentials.
- Initial deployment is simple; PostgreSQL/Redis can be added without service decomposition.
- Cross-module interfaces require deliberate versioning and may add small adapter overhead.
