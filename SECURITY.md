# Security

Report a vulnerability privately via GitHub's [Security Advisories](../../security/advisories/new) — please don't open a public issue for anything exploitable.

In scope: command execution declared in `.proofrun.yml`, the binary download/checksum path in `action.yml`, the local signing key / receipt integrity in `internal/receipt`, and STALE/fingerprint determination (`internal/git`, the fingerprint comparison in `internal/receipt`) — any path that can cause an unobserved or stale result to be reported as PASS is the most severe class of bug this project can have. See the README's "Tamper-evident receipts" section for what the local HMAC does and doesn't guarantee — some of those limits are known and intentional, not bugs.
