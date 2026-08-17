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
remote attestation, public-key/distributed signing, encryption, or OIDC, no web UI, no
full coding agent, no AI/LLM judging code correctness, no auto-fix, no MCP server, no
telemetry, no database or server component. v0.3's local HMAC signing (see below) is
deliberately limited to detecting casual receipt edits on a single machine — it is not
portable, remotely verifiable evidence, and adding any of the above needs its own
product decision, not a quiet expansion of what "signing" already means here.

## Tamper-evident receipts (`internal/receipt/sign.go`, `secret.go`, added v0.3)

Every stored `CheckResult` carries an HMAC-SHA256 signature under a random key
generated on first use and kept at `.proofrun/secret` — kept out of git on a
best-effort basis via the repository-local `.git/info/exclude` (see
`internal/git.EnsureIgnored`; nothing stops a user from `git add -f`-ing it anyway,
but if the key ever does end up git-tracked, `LoadOrCreateSecret` refuses to trust
it rather than silently signing with a key anyone who cloned the repo already
knows). ProofRun itself never transmits the key anywhere. `Save` signs; `Load`
verifies and silently drops any entry that doesn't check out (never signed, wrong
key, or edited after signing) — that's the same "not present in Checks" path that
already produces `NOT RUN`, so no fifth status exists for this.

**Read this before assuming it guarantees more than it does:**

- **Tamper-evident, not tamper-proof.** The threat this closes is a naive hand-edit
  of `receipt.json` (or an AI agent that doesn't know signing exists trying the same
  thing) — not a sophisticated attacker. Anyone who can read `.proofrun/secret` can
  forge a signature that verifies perfectly; that's an inherent limit of any
  local-only integrity scheme, not a gap to "fix" without fundamentally changing what
  this is.
- **Machine-local only, not portable evidence.** A `receipt.json` copied to another
  machine (or restored from a backup on the same machine) verifies only if
  `.proofrun/secret` travels with it. This was never meant to be something you hand
  someone else and say "trust this" — the GitHub Action's independent re-run remains
  the only thing that produces evidence a third party should trust.
- **Does not defend against rollback/replay.** A *genuinely* signed `receipt.json`
  from an earlier real run, restored later against a working tree that happens to
  have the identical fingerprint again (e.g. code reverted to a prior state), verifies
  correctly — the signature only proves "this machine really produced this at some
  point," not "this is the most recent run." STALE detection catches a fingerprint
  that no longer matches; it was never designed to catch a fingerprint that matches
  again by coincidence or reversion. Don't describe this feature as replay-proof.
- **A lost or corrupted key regenerates silently**, and every receipt signed under
  the old key stops verifying — this is the correct, conservative behavior (see
  `secret.go`'s doc comment), not a bug to route around.
- **Pre-v0.3 (unsigned) receipts read as `NOT RUN`, with no migration path.** Re-run
  the check; there is nothing else to do about an old, unsigned entry.
- **An entry whose signature fails to verify, if its name isn't declared in the
  current `.proofrun.yml`, disappears from `status` output entirely rather than
  showing `NOT RUN`** — `evaluateAll` only ever learns a name exists from config or
  from what's still in `Checks` after `Load` has already dropped invalid entries, so
  an orphaned tampered check (one that was `proofrun run`, never declared in config)
  has no other trace to surface. This is a deliberate simplicity choice, not a
  security gap — it never produces a PASS, only a check going silent — but don't be
  surprised if the count of visible checks briefly drops after tampering with
  something outside `.proofrun.yml`'s declared set.
- The GitHub Action's trust model is **unaffected** by any of this: it never reads a
  checked-out `receipt.json` in the first place (`rm -rf .proofrun/` before
  `run-all`), so it doesn't lean on local signing for anything.

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

This release-prep commit creates an intentional bootstrap gap: `dogfood.yml` uses the
branch's local `action.yml`, which now tries to download the new version before that
version has been tagged or published. Its three `verify` jobs therefore fail with a
release-asset HTTP 404 by construction. For a release-prep PR whose diff is limited to
the required release metadata changes — the `pin_version` update, and, when this release
starts a new compatibility line, the `.github/floating-major-tag` bump described below —
whose three `test` jobs pass, and whose `verify` logs fail only at that expected download,
use an administrator merge (or a direct push) instead of waiting for `verify` to turn
green. After the tag publishes the assets, re-run dogfood against the released version and
require all three `verify` jobs to pass; this exception applies only to the pre-release
bootstrap commit, not to ordinary changes.

**Floating major-tag policy.** `v1` is a promise to anyone who pins `uses:
yebiguo/proofrun@v1`: every version it moves to stays compatible with what `v1`
meant when they started depending on it. v0.2→v0.3 already shipped one
incompatible change of this shape (unsigned pre-v0.3 receipts read as `NOT RUN`
after upgrading, with no migration path — see "Tamper-evident receipts" above);
that was absorbable because `v1` didn't exist as a public dependency yet. The
next time a change of that shape ships *after* `v1` has real consumers, it must
get its own floating tag (`v2`) instead of being folded into `v1` — the same
convention `actions/checkout` and most of the Marketplace follow.

This isn't just something to remember at release time: `release.yml`'s
`float-tag` job reads which tag to move from `.github/floating-major-tag`
(currently `v1`) rather than hardcoding it, precisely because "is this release
compatible with what the floating tag's consumers already depend on" is a
maintainer judgment call that can't be inferred from a `vX.Y.Z` tag by pattern
matching — the CLI's own semver and the Action's floating-tag compatibility
line are tracked separately on purpose. **Part of release-prep for a breaking
release is bumping that file to `v2` in the same commit as the `pin_version`
bump, before tagging** — that starts a new floating tag and leaves the old one
permanently pinned at its last compatible release, instead of `float-tag`
silently carrying existing `@v1` consumers into the breaking change. Forgetting
this step doesn't fail loudly: `float-tag` will happily keep moving whatever
`.github/floating-major-tag` currently says, breaking change or not.
