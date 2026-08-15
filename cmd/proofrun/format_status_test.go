package main

import (
	"testing"

	"github.com/yebiguo/proofrun/internal/receipt"
)

// formatStatus is what a human actually reads to decide whether a check
// passed — a swapped word or wrong branch here would be invisible to every
// other test in this package, since they only assert on exit codes, not on
// the text status.go prints. This test exists specifically to catch that.
func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name string
		eval receipt.Evaluation
		want string
	}{
		{
			name: "pass",
			eval: receipt.Evaluation{
				Status: receipt.Pass,
				Stored: &receipt.CheckResult{Status: receipt.StatusPass, ExitCode: 0, DurationMS: 42},
			},
			want: "PASS    (exit 0, 42ms)",
		},
		{
			name: "fail",
			eval: receipt.Evaluation{
				Status: receipt.Fail,
				Stored: &receipt.CheckResult{Status: receipt.StatusFail, ExitCode: 1, DurationMS: 7},
			},
			want: "FAIL    (exit 1, 7ms)",
		},
		{
			name: "stale after a pass",
			eval: receipt.Evaluation{
				Status: receipt.Stale,
				Stored: &receipt.CheckResult{Status: receipt.StatusPass, ExitCode: 0, DurationMS: 100},
			},
			want: "STALE   (last run: pass, exit 0 — code changed since)",
		},
		{
			name: "stale after a fail",
			eval: receipt.Evaluation{
				Status: receipt.Stale,
				Stored: &receipt.CheckResult{Status: receipt.StatusFail, ExitCode: 2, DurationMS: 5},
			},
			want: "STALE   (last run: fail, exit 2 — code changed since)",
		},
		{
			name: "not run, no note",
			eval: receipt.Evaluation{Status: receipt.NotRun},
			want: "NOT RUN",
		},
		{
			name: "not run, with note (e.g. wrong-command case)",
			eval: receipt.Evaluation{Status: receipt.NotRun, Note: "recorded for a different command"},
			want: "NOT RUN (recorded for a different command)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatStatus(tc.eval)
			if got != tc.want {
				t.Errorf("formatStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}
