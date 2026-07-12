# Threat model

## Assets

```text
Notion session cookie
MCP/API tokens
private prompts and project files
memory database
MCP tool results
approval records
redacted protocol fixtures
account routing metadata
```

## Trust boundaries

1. Client/Codex to loopback gateway.
2. Gateway to authorized Notion transport.
3. Gateway to local SQLite and secret/config storage.
4. Gateway to local/remote MCP servers.
5. Gateway to web research providers.
6. Operator to diagnostics, exports, and backups.

## Core controls

| Threat | Current control | Planned hardening |
|---|---|---|
| Session credential leak | Do not log credentials; sanitizer/fixture rules | OS secret store or secure local config ACL validation |
| Prompt/tool result leak | Sanitization before memory/indexing | Encryption-at-rest and retention controls |
| Cross-project memory leak | Exact user/project query filters | Explicit user/global promotion model and audit |
| Unsafe MCP tool | Allowlist + approval-required tools | Full permission profiles and approval UI |
| Direct MCP endpoint exposure | Loopback check + optional API key | Mandatory auth outside loopback, TLS/reverse proxy guidance |
| Malicious MCP server | Explicit config + policy filter | Trust registry, signatures, process sandboxing |
| Upstream protocol drift | Fixture-backed decoder tests | Compatibility matrix and live canary tests |
| Rate limit/account failure | Local errors and health data | Explicit account pool/cooldown routing |
| Web prompt injection | Not yet full provider | Source trust tiers and untrusted-data framing |

## Non-goals

```text
No bypass of MFA, quotas, billing, bans, CAPTCHA, or account restrictions.
No hidden credential collection.
No hidden chain-of-thought capture.
No automatic account creation or stealth account rotation.
```
