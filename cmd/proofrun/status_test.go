package main

import (
	"os/exec"
	"testing"

	"github.com/yebiguo/proofrun/internal/config"
)

// A missing or emptied .proofrun.yml must make `status --strict` fail, not
// silently pass — an empty check set trivially satisfies "nothing is
// failing", and treating that as success would let a PR delete or empty out
// .proofrun.yml and get a green gate with zero checks actually run.

func TestStatusStrict_MissingConfig_ExitsNonZero(t *testing.T) {
	bin := buildProofrun(t)
	dir := newTestRepo(t)

	cmd := exec.Command(bin, "status", "--strict")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("status --strict with no .proofrun.yml exited 0, want nonzero\noutput:\n%s", out)
	}
}

func TestStatusStrict_EmptyChecks_ExitsNonZero(t *testing.T) {
	bin := buildProofrun(t)
	dir := newTestRepo(t)

	if err := (&config.Config{Checks: map[string]config.Check{}}).Save(dir); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "status", "--strict")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("status --strict with .proofrun.yml declaring zero checks exited 0, want nonzero\noutput:\n%s", out)
	}
}

func TestStatus_MissingConfig_NonStrictStillExitsZero(t *testing.T) {
	bin := buildProofrun(t)
	dir := newTestRepo(t)

	cmd := exec.Command(bin, "status")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status (non-strict) with no .proofrun.yml exited nonzero, want 0\noutput:\n%s", out)
	}
}
