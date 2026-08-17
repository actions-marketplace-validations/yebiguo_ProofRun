# Security

Report a vulnerability privately via GitHub's [Security Advisories](../../security/advisories/new) — please don't open a public issue for anything exploitable.

In scope: command execution declared in `.proofrun.yml`, the binary download/checksum path in `action.yml`, and the local signing key / receipt integrity in `internal/receipt`. See the README's "Tamper-evident receipts" section for what the local HMAC does and doesn't guarantee — some of those limits are known and intentional, not bugs.
