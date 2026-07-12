# Historical Phase 2 research harness report

> This is a historical record. Use [Protocol research](protocol-research.md), [Live validation](live-validation.md), and [Current status](current-status.md) for current guidance.

Phase 2 established the safe fixture boundary:

```text
strict fixture envelope
credential-shaped key/value rejection
normalized downstream stream harness
fixture probing CLI
fuzz and regression foundations
```

Later work added observed live decoder regressions for `patch-start` and `patch-sync` stream offsets. This report must not be read as a claim that live transport is absent.
