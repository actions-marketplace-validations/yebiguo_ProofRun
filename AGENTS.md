# AGENTS.md

## What this project is

ProofRun is a local verification receipt for AI coding agents. It binds the result of a
command execution (exit code, duration) to the exact git state (HEAD commit + working-tree
diff fingerprint) at the moment it ran, and stores that as `receipt.json`. If the code
changes by even one byte, the receipt is immediately STALE.

This is a single-maintainer open-source CLI tool. It does not process any user-sensitive
data (no telemetry, no network calls, no accounts). It is built for public release.

Go, single-file binary. No server, no database, no LLM calls, ever, in this tool's core
logic — that is a product guarantee, not an implementation detail. See `.proofrun.yml` /
`.proofrun/receipt.json` for the on-disk formats.

## Product philosophy (read before changing status logic)

1. ProofRun never judges whether code is correct — only whether a check command was
   actually observed executing against the exact code that exists right now.
2. Status can only come from an observed execution, never from inference. There are
   exactly four statuses: `PASS`, `FAIL`, `STALE`, `NOT RUN`. Do not add a fifth
   (especially not an "INFERRED" or AI-guessed status) without an explicit product
   decision — this is the core trust guarantee of the whole tool.
3. When in doubt, report conservatively (`NOT RUN` / `STALE`) rather than optimistically
   (`PASS`). A false PASS is a severity-critical bug here, not a normal bug — it breaks
   the reason this tool exists.

## Collaboration tier

**Exploration tier.** You may work autonomously and push branches / open Draft PRs
without asking first. This is a young, low-stakes, pre-1.0 project.

## High-risk changes (slow down, be extra careful here)

- STALE determination logic (`internal/git`, the fingerprint comparison in
  `internal/receipt`) — this is the entire product. Needs strong test coverage,
  including boundary cases like whitespace-only diffs, line-ending differences, and
  untracked files.
- `receipt.json` schema (`internal/receipt`) — once released, schema changes must
  consider backward compatibility (old receipts should not crash new binaries).

## Before calling anything done

- `go test ./...` must pass.
- Cross-compile for the three release targets (windows/amd64, darwin/amd64 or arm64,
  linux/amd64) to confirm nothing platform-specific broke:
  `GOOS=windows GOARCH=amd64 go build ./...`, `GOOS=darwin GOARCH=arm64 go build ./...`,
  `GOOS=linux GOARCH=amd64 go build ./...`
- For anything touching STALE detection: manually verify by running a check, editing a
  tracked file, and confirming `proofrun status` reports STALE — don't rely on unit
  tests alone for this one.
- Verify your own work with ProofRun itself, not just raw `go build`/`go test`/`go vet`:
  build the binary from source and run `proofrun run-all` (or `proofrun run <name> --
  <cmd>`) against this repo before calling a change done, so the receipt bound to the
  current commit reflects what was actually verified. A tool whose own maintainers
  don't dogfood it has no standing to ask anyone else to trust it.

## Explicitly out of scope (do not add without a product decision)

No INFERRED status, no parsing of test/build output content (exit code only), no
signing/encryption/OIDC, no web UI, no full coding agent, no AI/LLM judging code
correctness, no auto-fix, no MCP server, no telemetry, no database or server component.

## GitHub Action (`action.yml`, added v0.2)

Independently re-runs whatever `.proofrun.yml` declares against the exact PR head
commit — never trusts a `receipt.json` checked out from the PR branch (`rm -rf
.proofrun/` runs before `run-all`). It does its own authoritative checkout (`ref:
github.event.pull_request.head.sha`) rather than trusting the caller's, because the
default `pull_request`-triggered checkout lands on GitHub's synthetic merge-preview
commit, not the real head — see the assert step in action.yml for why that isn't
just documented around.

It does **not** protect `.proofrun.yml` itself from being weakened by the same PR
that changes the code — it only warns (a non-blocking `::warning::` annotation) when
that file differs from the PR's base branch. Don't remove that warning or turn it
into anything that looks like a guarantee it isn't; if `.proofrun.yml`-tampering
protection is ever added, it needs its own product decision, not a quiet expansion
of this warning's scope.

**Release-prep step, required before tagging any real version (e.g. `v0.3.0`):** edit
action.yml's "Download proofrun" step so its `pin_version="..."` literal reads exactly
the version being released (e.g. `pin_version="v0.3.0"`), commit that change, *then* tag
the resulting commit. Don't assume what the current value already says — check
action.yml itself, since this step's whole point is that the value always changes at
every release. This makes the tagged commit self-consistent — pinning by that tag, by
the floating `v1` tag (once release.yml's `float-tag` job moves it there), or by that
commit's full SHA all resolve to identical action.yml content with the identical binary
version baked in. Skipping this step means the release's action.yml still says
whatever version it last shipped with, so it downloads a binary that doesn't match the
version it was actually released as.
