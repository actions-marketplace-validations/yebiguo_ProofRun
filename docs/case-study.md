# Case study: a tool built to catch false PASS shipped one

ProofRun exists to answer one question: did a check actually run against the exact code you have right now? Before the first release, an independent adversarial review of this codebase found three separate ways to make it answer that question wrongly. This is the most interesting one — a false `PASS` that required no malicious intent and no receipt tampering, just an ordinary shell quoting mistake.

## The setup

`.proofrun.yml` lets a project declare the command a check is supposed to run:

```yaml
checks:
  test:
    command: go test -run TestCritical ./...
    required: true
```

At the time, `command` was a single string. When `proofrun status --strict` evaluated a stored result, it had to decide whether the command that was *actually* run matched what was *declared*. The comparison it used:

```go
actual := strings.Join(storedCommand, " ")
if actual != declaredCommand {
    // NOT RUN — command mismatch
}
```

That looks reasonable. It isn't.

## The bug

A real command is a list of arguments (an argv), not a string. `proofrun run <name> -- <cmd>` receives that argv straight from the shell that invoked it — and two *different* argvs can join into the *same* string:

```go
[]string{"go", "test", "-run", "TestCritical", "./..."}   // 5 elements
[]string{"go", "test", "-run", "TestCritical ./..."}      // 4 elements
```

Both produce the string `"go test -run TestCritical ./..."`. But they are not the same command. The second one is what you get if a stray quote swallows `./...` into the `-run` argument — an easy, completely non-malicious mistake to make on a command line. Run it for real:

```console
$ go test -run TestCritical ./...
--- FAIL: TestCritical (0.00s)
    main_test.go:6: this test should always fail if it actually runs
FAIL

$ go test -run "TestCritical ./..."
testing: warning: no tests to run
PASS
```

The correctly-quoted command runs the test and fails, as it should. The mis-quoted one matches zero tests and exits `0` — a real, successful process exit, not a crash. Fed through `strings.Join`, ProofRun couldn't tell them apart. The exact exploit:

```bash
proofrun run test -- go test -run "TestCritical ./..."
proofrun status --strict   # exited 0
```

A check that was never really run, reporting PASS, blocking nothing. For a tool whose only job is "prove this was actually verified," that is the one failure mode that isn't allowed to exist.

## Finding it

This surfaced during an independent read-only review pass on the codebase, not from internal testing — the existing test suite covered the *intended* comparison logic correctly and had no reason to try a misquoted argument. Before changing anything, the fix was verified against a real throwaway repo containing a test that fails when actually executed, confirming both that the mis-quoted command really did exit 0 and that `status --strict` really did accept it.

## The fix

Patching the string comparison — re-tokenizing with a shell-aware parser — was considered and rejected. ProofRun targets Windows, macOS, and Linux, and shell quoting rules aren't unified across them; any parser would just be guessing at a different set of edge cases. The real fix was to stop comparing strings at all.

`.proofrun.yml`'s `command` field became a YAML list — an argv, matching what actually executes:

```yaml
checks:
  test:
    command: [go, test, -run, TestCritical, ./...]
    required: true
```

And the comparison became element-for-element:

```go
if !slices.Equal(storedCommand, declaredCommand) {
    // NOT RUN — command mismatch
}
```

`[go test -run TestCritical ./...]` and `[go test -run "TestCritical ./..."]` are now unambiguously different slices. There is no flattening step left for a quoting accident to hide inside.

This was a breaking schema change, made deliberately rather than patched around: the project was still private and pre-1.0, with no external consumers of the old string format yet — the cheapest point at which to fix it that it will ever be.

## Verifying the fix actually fixed it

Re-running the exact exploit against the rebuilt binary:

```console
$ proofrun run test -- go test -run "TestCritical ./..."
testing: warning: no tests to run
PASS

$ proofrun status --strict
test    NOT RUN (last recorded run used ["go" "test" "-run" "TestCritical ./..."],
                  but .proofrun.yml declares ["go" "test" "-run" "TestCritical" "./..."])
$ echo $?
1
```

Full commit: [`261f41f`](https://github.com/yebiguo/proofrun/commit/261f41fc897a42fd7991a75d310a43ca783c6b54).

## The other two

The same review round also caught: a check declared with an empty command (`command: []`) being indistinguishable downstream from "not declared at all," letting any command satisfy a `required: true` check; and the more basic version of this same bug, where the executed command wasn't checked against the declared one *at all* — `proofrun run test -- true` would satisfy a check declared as `go test ./...`. Both are closed the same way: the config layer now rejects empty commands outright, and every evaluation path compares real argvs, never strings.

## The takeaway

None of these three were found by the author. They were found by treating "we built the tool that proves things really happened" as a claim that itself needed proving — by adversarially trying to break the exact guarantee the tool exists to make, before anyone else could. That's the review this project's own design says should never be skipped, applied to itself.
