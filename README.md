# ProofRun

> ProofRun is a local verification receipt for AI coding agents — it doesn't judge whether code is correct, it only proves which checks actually ran against the exact code you have right now.

```bash
# install (see Releases for other platforms)
curl -L https://github.com/yebiguo/ProofRun/releases/latest/download/proofrun_linux_amd64.tar.gz | tar xz

# use
proofrun init                    # writes .proofrun.yml
proofrun run test -- pytest      # actually executes pytest, binds the result to your current code
proofrun status                  # PASS / FAIL / STALE / NOT RUN, per check
```

**An AI coding agent saying "all tests pass" is a claim, not a fact.** By the time it says that sentence — especially after several more rounds of edits — the code may no longer be the code that was tested, and the agent often doesn't know it. ProofRun doesn't try to judge whether the code is *good*. It answers one narrower, verifiable question: **did a check command actually run, against the exact code that exists right now?**

```
$ proofrun run test -- pytest
...
test: pass (exit 0, 1841ms)

$ proofrun status
test                 PASS    (exit 0, 1841ms)

# an agent edits a file after this point, without re-running the test

$ proofrun status
test                 STALE   (last run: pass, exit 0 — code changed since)
```

That's the whole trick: a check's result is cryptographically bound to your git HEAD and working-tree diff at the moment it ran. Change one byte of tracked or untracked code afterward, and the result flips to `STALE` automatically — no one has to remember to ask "is this still valid?"

## Why this, not just trusting the agent

- **No LLM calls, anywhere.** ProofRun doesn't use AI to verify AI. It runs a real subprocess and reads its real exit code — that's it.
- **Only four statuses, and none of them are a guess:** `PASS`, `FAIL`, `STALE`, `NOT RUN`. Every one of them comes from either an observed execution or the documented absence of one. There is no fifth "probably fine" status.
- **Fully offline.** Zero network requests, zero telemetry, zero accounts.

## Commands

```bash
proofrun init                      # generate .proofrun.yml in the current directory
proofrun run <check-name> -- <cmd> # run <cmd> for real, record exit code + duration bound to current git state
proofrun status [--strict]         # show status of every check; --strict exits non-zero if any required check isn't PASS
proofrun report [--json]           # human-readable or machine-readable full report
```

## Config: `.proofrun.yml`

```yaml
checks:
  test:
    command: pytest
    required: true
  build:
    command: npm run build
    required: true
  lint:
    command: ruff check .
    required: false
```

`required: true` is what makes a check block `proofrun status --strict` — useful as a pre-commit hook or CI gate that refuses to trust a receipt an agent (or a human) claims but never actually re-ran.

## What ProofRun does not do (on purpose)

It does not parse test output, does not judge code quality, does not auto-fix anything, does not call any LLM, and does not run in CI yet (a v0.2 GitHub Action is planned, which will re-run checks itself rather than trusting a receipt generated on someone's laptop). See [AGENTS.md](AGENTS.md) for the full list of what's deliberately out of scope for v0.1.

## How the fingerprint works

Every check result is bound to a `Fingerprint`: your current git `HEAD` commit, plus a SHA-256 hash of `git diff HEAD` (staged and unstaged changes to tracked files) combined with the contents of any untracked, non-gitignored files. `proofrun status` recomputes that fingerprint on every run and compares it against what's stored in `.proofrun/receipt.json` (a local, gitignored file) — any mismatch, down to a single changed space or one new file, is reported as `STALE`.

## License

MIT
