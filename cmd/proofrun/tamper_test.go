package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yebiguo/proofrun/internal/config"
	"github.com/yebiguo/proofrun/internal/receipt"
)

// TestStatusStrict_HandEditedReceipt_NeverShowsForgedPass is the
// CLI/subprocess-level version of the adversarial scenario proven at the
// unit level in internal/receipt (TestLoad_DropsHandEditedEntryEvenWithMatchingFingerprint)
// and, before that, manually against a real build during v0.3 Day 2. It
// exists to catch anything a pure in-process Go test can't: argument
// parsing, exit code propagation, and output formatting all going through
// the actual compiled binary exactly the way a real user's shell — or a
// pre-commit hook that only ever runs `proofrun status --strict`, never an
// Action, never a re-run — would invoke it.
//
// Mirrors v0.2's PR #5 methodology (forge a receipt with a fingerprint
// that matches perfectly, don't rely on staleness to save you) but targets
// the path v0.2 didn't close: no GitHub Action, no rm -rf .proofrun/, just
// a local `status --strict` reading whatever's on disk.
func TestStatusStrict_HandEditedReceipt_NeverShowsForgedPass(t *testing.T) {
	bin := buildProofrun(t)
	dir := newTestRepo(t)

	var failCmd []string
	if runtime.GOOS == "windows" {
		failCmd = []string{"cmd", "/C", "exit", "/B", "1"}
	} else {
		failCmd = []string{"sh", "-c", "exit 1"}
	}

	cfg := &config.Config{Checks: map[string]config.Check{
		"test": {Command: failCmd, Required: true},
	}}
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}

	// Record a genuine, signed FAIL — this is what a real, honest run
	// against this exact commit actually produces.
	runAll := exec.Command(bin, "run-all")
	runAll.Dir = dir
	out, err := runAll.CombinedOutput()
	if err == nil {
		t.Fatalf("run-all unexpectedly succeeded against a command that exits 1:\n%s", out)
	}

	receiptPath := filepath.Join(dir, receipt.DirName, receipt.FileName)
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("reading receipt.json after run-all: %v", err)
	}
	var onDisk receipt.Receipt
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("receipt.json is not valid JSON: %v\n%s", err, raw)
	}
	cr, ok := onDisk.Checks["test"]
	if !ok {
		t.Fatal("run-all did not record a result for \"test\"")
	}
	if cr.Status != receipt.StatusFail {
		t.Fatalf("recorded status = %q, want fail — test setup is wrong", cr.Status)
	}
	originalSignature := cr.Signature

	// Hand-edit exactly the way a naive attacker (or an AI agent that
	// doesn't know signing exists) would: flip the fields that matter,
	// leave the signature untouched.
	cr.Status = receipt.StatusPass
	cr.ExitCode = 0
	onDisk.Checks["test"] = cr
	tampered, err := json.MarshalIndent(&onDisk, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if onDisk.Checks["test"].Signature != originalSignature {
		t.Fatal("test setup bug: the signature itself was accidentally changed, this must stay untouched to reproduce a real hand-edit")
	}

	statusCmd := exec.Command(bin, "status", "--strict")
	statusCmd.Dir = dir
	statusOut, statusErr := statusCmd.CombinedOutput()

	if statusErr == nil {
		t.Fatalf("status --strict exited 0 against a hand-edited receipt claiming PASS — the forged PASS was trusted\noutput:\n%s", statusOut)
	}
	// "PASS" would also match as a substring of "NOT RUN"'s neighboring
	// text in principle, but formatStatus's actual output ("PASS    (exit"
	// vs "NOT RUN") never puts the literal word "PASS" anywhere near a
	// legitimate NOT RUN line, so a plain substring check is precise enough
	// here without needing to parse the table.
	if bytes.Contains(statusOut, []byte("PASS")) {
		t.Fatalf("status --strict output mentions PASS for a hand-edited receipt — expected NOT RUN\noutput:\n%s", statusOut)
	}
	if !bytes.Contains(statusOut, []byte("NOT RUN")) {
		t.Fatalf("status --strict output does not show NOT RUN for the tampered check as expected\noutput:\n%s", statusOut)
	}
}

// TestStatus_TamperedUndeclaredCheck_DisappearsRatherThanShowingNotRun locks
// in a deliberate, documented (AGENTS.md, "Tamper-evident receipts") edge
// case: receipt.Load already drops any entry whose signature doesn't
// verify before evaluateAll (status.go) ever sees it, and evaluateAll only
// learns a check's name exists from .proofrun.yml or from whatever
// survived in the receipt. A check that was `proofrun run` without ever
// being declared in .proofrun.yml, then tampered with, has no other trace
// to surface — it disappears from `status` output entirely rather than
// showing NOT RUN. This never produces a false PASS (the direction that
// actually matters), but it's a real, easy-to-get-backwards behavior that
// deserves a test pinning it down rather than an accident nobody notices
// until the docs and the code disagree.
func TestStatus_TamperedUndeclaredCheck_DisappearsRatherThanShowingNotRun(t *testing.T) {
	bin := buildProofrun(t)
	dir := newTestRepo(t)
	// Deliberately no .proofrun.yml anywhere in dir.

	var okCmd []string
	if runtime.GOOS == "windows" {
		okCmd = []string{"cmd", "/C", "exit", "/B", "0"}
	} else {
		okCmd = []string{"sh", "-c", "exit 0"}
	}

	runCmd := exec.Command(bin, append([]string{"run", "adhoc", "--"}, okCmd...)...)
	runCmd.Dir = dir
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("proofrun run adhoc: %v\n%s", err, out)
	}

	receiptPath := filepath.Join(dir, receipt.DirName, receipt.FileName)
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk receipt.Receipt
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	cr, ok := onDisk.Checks["adhoc"]
	if !ok {
		t.Fatal("run did not record a result for \"adhoc\"")
	}

	// Corrupt the signature itself this time (rather than a content field)
	// — either way invalidates it, and this is the simplest way to do so
	// without needing to know the local key.
	cr.Signature = "0000000000000000000000000000000000000000000000000000000000000000"
	onDisk.Checks["adhoc"] = cr
	tampered, err := json.MarshalIndent(&onDisk, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}

	statusCmd := exec.Command(bin, "status")
	statusCmd.Dir = dir
	out, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status (non-strict) should exit 0 here, got: %v\n%s", err, out)
	}

	if bytes.Contains(out, []byte("adhoc")) {
		t.Fatalf("status output still mentions \"adhoc\" — expected it to disappear entirely once its only evidence (the receipt entry) failed verification\noutput:\n%s", out)
	}
	if !bytes.Contains(out, []byte("no checks found")) {
		t.Fatalf("expected the documented \"no checks found\" message (nothing declared, nothing left to evaluate), got:\n%s", out)
	}
}
