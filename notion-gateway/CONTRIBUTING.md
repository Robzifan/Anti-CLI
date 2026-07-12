# Contributing

1. Do not commit real Notion protocol data or credentials.
2. Keep protocol observations in the isolated adapter and sanitized fixtures.
3. Run `make fmt lint test test-race test-fuzz build security`.
4. Add tests for behavior and document compatibility claims precisely.
5. Use small changes and include an ADR for architectural boundary changes.
