# Security policy

## Reporting

Do not open a public issue containing credentials, raw captures, private prompts, account identifiers, or exploitable details. Use the repository host's private security advisory feature. A dedicated disclosure mailbox will be published before the first public release.

## Supported versions

There is no production-supported release yet. The main branch is an early scaffold.

## Credential handling

Never provide cookies, `token_v2`, bearer tokens, API keys, `auth.json`, or raw browser captures in issues, chat, fixtures, or logs. The project will not help bypass access, billing, MFA, CAPTCHA, suspension, quotas, or other server controls.

See [docs/threat-model.md](docs/threat-model.md).
