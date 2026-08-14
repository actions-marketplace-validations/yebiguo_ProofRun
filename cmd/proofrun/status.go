package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/yebiguo/proofrun/internal/config"
	"github.com/yebiguo/proofrun/internal/git"
	"github.com/yebiguo/proofrun/internal/receipt"
)

var statusStrict bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of all checks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		evals, requiredByName, err := evaluateAll(dir)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		if len(evals) == 0 {
			fmt.Fprintln(out, "no checks found — run `proofrun init` and `proofrun run <name> -- <cmd>`")
			// An empty check set trivially satisfies "nothing is failing" —
			// under --strict that would silently gate on nothing, exactly
			// the false-confidence failure mode this project exists to
			// prevent. A missing/emptied .proofrun.yml must block, not pass.
			if statusStrict {
				os.Exit(1)
			}
			return nil
		}

		blocking := false
		for _, e := range evals {
			fmt.Fprintf(out, "%-20s %s\n", e.Name, formatStatus(e))
			if e.Status != receipt.Pass && requiredByName[e.Name] {
				blocking = true
			}
		}

		if statusStrict && blocking {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusStrict, "strict", false, "exit non-zero if any required check is not PASS")
	rootCmd.AddCommand(statusCmd)
}

// evaluateAll computes the display-time status of every check known either
// from .proofrun.yml (so never-run checks still show as NOT RUN) or from
// the receipt (so a check that was run without ever being declared in
// config still shows up). Returns evaluations sorted by name, plus which
// names are "required" per config (defaults to false for undeclared
// checks — declaring `required: true` in .proofrun.yml is what opts a
// check into blocking `status --strict`).
func evaluateAll(dir string) ([]receipt.Evaluation, map[string]bool, error) {
	head, err := git.Head(dir)
	if err != nil {
		return nil, nil, err
	}
	diff, err := git.DiffFingerprint(dir, receipt.DirName)
	if err != nil {
		return nil, nil, err
	}
	current := receipt.Fingerprint{Head: head, DiffSHA256: diff}

	r, err := receipt.Load(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("loading receipt: %w", err)
	}

	names := map[string]bool{}
	required := map[string]bool{}
	expectedCommand := map[string][]string{}
	if config.Exists(dir) {
		cfg, err := config.Load(dir)
		if err != nil {
			return nil, nil, err
		}
		for name, check := range cfg.Checks {
			names[name] = true
			required[name] = check.Required
			expectedCommand[name] = check.Command
		}
	}
	for name := range r.Checks {
		names[name] = true
	}

	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	// EvaluateAgainstCommand (not the plainer Evaluate) matters here: a
	// check declared in .proofrun.yml with a specific command must never
	// read as PASS/FAIL from a receipt recorded for some other command —
	// e.g. `proofrun run test -- true` satisfying a config that declares
	// `test: go test ./...`. See internal/receipt.EvaluateAgainstCommand.
	evals := make([]receipt.Evaluation, 0, len(sorted))
	for _, name := range sorted {
		evals = append(evals, receipt.EvaluateAgainstCommand(r, name, current, expectedCommand[name]))
	}
	return evals, required, nil
}

func formatStatus(e receipt.Evaluation) string {
	switch e.Status {
	case receipt.Pass:
		return fmt.Sprintf("PASS    (exit %d, %dms)", e.Stored.ExitCode, e.Stored.DurationMS)
	case receipt.Fail:
		return fmt.Sprintf("FAIL    (exit %d, %dms)", e.Stored.ExitCode, e.Stored.DurationMS)
	case receipt.Stale:
		return fmt.Sprintf("STALE   (last run: %s, exit %d — code changed since)", e.Stored.Status, e.Stored.ExitCode)
	default:
		if e.Note != "" {
			return fmt.Sprintf("NOT RUN (%s)", e.Note)
		}
		return "NOT RUN"
	}
}
