// Package ci orchestrates running every check declared in .proofrun.yml in
// one pass. It's the shared primitive behind `proofrun run-all` and the
// GitHub Action (see action.yml): both need "execute everything the config
// declares" rather than one caller-supplied command at a time.
//
// Unlike `proofrun run <name> -- <cmd>`, there is no separate CLI-supplied
// command to compare against here — a check's command comes from exactly
// one place, .proofrun.yml — so the whole class of argv-vs-declaration
// mismatch that internal/receipt.EvaluateAgainstCommand guards against
// structurally cannot happen on this path.
package ci

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/yebiguo/proofrun/internal/config"
	"github.com/yebiguo/proofrun/internal/git"
	"github.com/yebiguo/proofrun/internal/receipt"
	"github.com/yebiguo/proofrun/internal/runner"
)

// CheckOutcome is one check's result from a RunAll pass.
type CheckOutcome struct {
	Name string
	// Result is only meaningful when Err is nil and TimedOut is false —
	// it's the real observed outcome, already saved to the receipt.
	Result   receipt.CheckResult
	TimedOut bool
	// Err is set when the command could not even be started (bad binary,
	// permissions, ...). That's not an observation of the check, so
	// nothing is recorded in the receipt for it — same rule as `run`.
	Err error
}

// Failed reports whether this outcome should count as a failure for the
// purposes of deciding RunAll's overall exit status.
func (o CheckOutcome) Failed() bool {
	return o.Err != nil || o.TimedOut || o.Result.ExitCode != 0
}

// RunAll executes every check declared in .proofrun.yml — or only those
// named in `only`, if `only` is non-empty — in alphabetical order, against
// one fingerprint of the current git state computed once up front (so all
// checks in the same run are bound to the identical commit+diff).
//
// The receipt is saved to disk after each check completes, not batched
// until the end: if the process is killed partway through (a CI job
// timeout, a terminated runner), whichever checks already finished stay
// durably recorded rather than being lost along with the ones in flight.
//
// RunAll always attempts every declared check, even after an earlier one
// fails or errors — a tool whose entire job is producing an accurate trust
// signal must never stop early and leave later checks silently unattempted;
// a partial run that silently skips checks is its own kind of false
// confidence.
//
// Returns an error before running anything if .proofrun.yml doesn't exist,
// or declares zero checks after `only` is applied: an empty check set
// trivially satisfies `status --strict` (there's nothing to fail), which
// would silently gate on nothing — exactly the false-confidence failure
// mode this project exists to prevent.
func RunAll(ctx context.Context, dir string, timeout time.Duration, only map[string]bool, stdout, stderr io.Writer) ([]CheckOutcome, error) {
	if !config.Exists(dir) {
		return nil, fmt.Errorf("%s not found in %s — run `proofrun init` first", config.FileName, dir)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Checks))
	for name := range cfg.Checks {
		if len(only) > 0 && !only[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		if len(only) > 0 {
			return nil, fmt.Errorf("no check in %s matches the requested --only name(s)", config.FileName)
		}
		return nil, fmt.Errorf("%s declares no checks — nothing to run", config.FileName)
	}

	head, err := git.Head(dir)
	if err != nil {
		return nil, err
	}
	diff, err := git.DiffFingerprint(dir, receipt.DirName)
	if err != nil {
		return nil, err
	}
	fp := receipt.Fingerprint{Head: head, DiffSHA256: diff}

	r, err := receipt.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loading receipt: %w", err)
	}

	outcomes := make([]CheckOutcome, 0, len(names))
	for _, name := range names {
		check := cfg.Checks[name]
		fmt.Fprintf(stdout, "\n=== %s: %s ===\n", name, strings.Join(check.Command, " "))

		res, runErr := runner.Run(ctx, dir, check.Command, timeout, stdout, stderr)
		switch {
		case runErr != nil:
			outcomes = append(outcomes, CheckOutcome{Name: name, Err: runErr})
			fmt.Fprintf(stdout, "%s: error (%v)\n", name, runErr)
			continue
		case res.TimedOut:
			outcomes = append(outcomes, CheckOutcome{Name: name, TimedOut: true})
			fmt.Fprintf(stdout, "%s: timed out\n", name)
			continue
		}

		cr := receipt.NewResult(check.Command, res.ExitCode, res.StartedAt, res.Duration, fp)
		r.Set(name, cr)
		if err := r.Save(dir); err != nil {
			return outcomes, fmt.Errorf("saving receipt after %q: %w", name, err)
		}

		outcomes = append(outcomes, CheckOutcome{Name: name, Result: cr})
		fmt.Fprintf(stdout, "%s: %s (exit %d, %dms)\n", name, cr.Status, cr.ExitCode, cr.DurationMS)
	}

	return outcomes, nil
}
