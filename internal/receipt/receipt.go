// Package receipt owns the on-disk receipt.json schema and the STALE
// determination logic — the core trust mechanism of ProofRun.
//
// A stored CheckResult only ever records one of two literal outcomes of a
// real execution: "pass" or "fail", derived from the process's exit code.
// STALE and NOT RUN are never written to disk — they are computed at read
// time by comparing a stored result's fingerprint against the caller's
// current fingerprint (see Evaluate). That split matters: only an observed
// execution can produce a stored result, but staleness is a fact about the
// present, not about what happened when the check last ran.
package receipt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// SchemaVersion identifies the receipt.json format.
const SchemaVersion = "proofrun/v1"

// DirName is the directory receipt.json lives in, relative to the project
// root. It is machine-local state and should not be committed to git.
const DirName = ".proofrun"

// FileName is the receipt file's name inside DirName.
const FileName = "receipt.json"

// Status is a literal, stored outcome of a real execution.
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
)

// Fingerprint is the git state a CheckResult is bound to.
type Fingerprint struct {
	Head       string `json:"head"`
	DiffSHA256 string `json:"diff_sha256"`
}

// Equal reports whether two fingerprints refer to the same git state.
func (f Fingerprint) Equal(other Fingerprint) bool {
	return f.Head == other.Head && f.DiffSHA256 == other.DiffSHA256
}

// CheckResult is one check's stored outcome.
type CheckResult struct {
	Status          Status      `json:"status"`
	Command         []string    `json:"command"`
	ExitCode        int         `json:"exit_code"`
	DurationMS      int64       `json:"duration_ms"`
	StartedAt       time.Time   `json:"started_at"`
	VerifiedAgainst Fingerprint `json:"verified_against"`
}

// Receipt is the full contents of receipt.json.
type Receipt struct {
	Schema string                 `json:"schema"`
	Checks map[string]CheckResult `json:"checks"`
}

// New returns an empty receipt with the current schema version.
func New() *Receipt {
	return &Receipt{Schema: SchemaVersion, Checks: map[string]CheckResult{}}
}

// NewResult builds the CheckResult for a single observed execution. exitCode
// 0 is StatusPass, anything else is StatusFail — that is the only rule;
// there is no other way to produce a stored status.
func NewResult(command []string, exitCode int, startedAt time.Time, duration time.Duration, fp Fingerprint) CheckResult {
	status := StatusFail
	if exitCode == 0 {
		status = StatusPass
	}
	return CheckResult{
		Status:          status,
		Command:         command,
		ExitCode:        exitCode,
		DurationMS:      duration.Milliseconds(),
		StartedAt:       startedAt.UTC(),
		VerifiedAgainst: fp,
	}
}

// Path returns the full path to receipt.json inside project root dir.
func Path(dir string) string {
	return filepath.Join(dir, DirName, FileName)
}

// Load reads receipt.json from dir. A missing file is not an error: it
// returns a fresh empty receipt, since "no receipt yet" is a normal,
// expected state (every check is NOT RUN).
func Load(dir string) (*Receipt, error) {
	data, err := os.ReadFile(Path(dir))
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}

	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if r.Checks == nil {
		r.Checks = map[string]CheckResult{}
	}
	return &r, nil
}

// Save writes the receipt to dir, creating .proofrun/ if needed.
func (r *Receipt) Save(dir string) error {
	if r.Schema == "" {
		r.Schema = SchemaVersion
	}
	if err := ensureDir(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(Path(dir), data, 0o644)
}

// Set records or overwrites a check's result.
func (r *Receipt) Set(name string, result CheckResult) {
	if r.Checks == nil {
		r.Checks = map[string]CheckResult{}
	}
	r.Checks[name] = result
}

// EvaluatedStatus is the display-time status of a check: what you see from
// `proofrun status`. Unlike Status, it is never written to disk — it is
// always recomputed from the stored CheckResult (if any) and the caller's
// current fingerprint. There are exactly four values; do not add a fifth
// (e.g. an inferred/guessed status) without revisiting AGENTS.md — the
// whole point of ProofRun is that this value only ever comes from an
// observed execution or the absence of one, never from inference.
type EvaluatedStatus string

const (
	Pass   EvaluatedStatus = "PASS"
	Fail   EvaluatedStatus = "FAIL"
	Stale  EvaluatedStatus = "STALE"
	NotRun EvaluatedStatus = "NOT RUN"
)

// Evaluation is one check's display-time status plus the stored result it
// was derived from, if any.
type Evaluation struct {
	Name   string
	Status EvaluatedStatus
	Stored *CheckResult
	// Note is an optional human-readable explanation for cases where
	// Status/Stored alone don't say why — e.g. why a stored result wasn't
	// accepted as evidence for this check. It is not a fifth status.
	Note string
}

// Evaluate compares the receipt's stored result for name (if any) against
// the caller-supplied current fingerprint:
//   - no stored result at all               -> NOT RUN
//   - stored result's fingerprint mismatches -> STALE (regardless of the
//     stored exit code — a stale PASS is not a PASS)
//   - fingerprint matches, stored status was pass -> PASS
//   - fingerprint matches, stored status was fail -> FAIL
func Evaluate(r *Receipt, name string, current Fingerprint) Evaluation {
	cr, ok := r.Checks[name]
	if !ok {
		return Evaluation{Name: name, Status: NotRun}
	}
	stored := cr
	if !cr.VerifiedAgainst.Equal(current) {
		return Evaluation{Name: name, Status: Stale, Stored: &stored}
	}
	if cr.Status == StatusPass {
		return Evaluation{Name: name, Status: Pass, Stored: &stored}
	}
	return Evaluation{Name: name, Status: Fail, Stored: &stored}
}

// EvaluateAgainstCommand is Evaluate, plus one more requirement: when
// expectedCommand is non-empty (the check is declared in .proofrun.yml with
// a specific command), the stored result's actual argv must equal it
// element-for-element.
//
// This closes a real gap: `proofrun run <name> -- <cmd>` lets the caller
// pass any command on the CLI, so without this check `proofrun run test --
// true` would satisfy a check declared in config as `test: go test ./...`
// — a zero-cost forged PASS that `status --strict` would happily treat as
// the real thing. A stored result for the wrong command is not evidence
// about the right one, so a mismatch can only ever report NOT RUN — never
// PASS or FAIL, even if the wrong command happened to exit non-zero.
//
// The comparison is element-for-element, never a joined/flattened string:
// joining is lossy. []string{"go","test","-run","TestCritical","./..."}
// and []string{"go","test","-run","TestCritical ./..."} are genuinely
// different commands (the second matches zero tests and exits 0 in most
// repos) but join to the identical string "go test -run TestCritical
// ./..." — a real false-PASS path if comparison ever flattens either side
// first. See internal/config.Check.Command's doc comment for the same
// reasoning from the config side.
func EvaluateAgainstCommand(r *Receipt, name string, current Fingerprint, expectedCommand []string) Evaluation {
	eval := Evaluate(r, name, current)
	if len(expectedCommand) == 0 || eval.Stored == nil {
		return eval
	}
	if !slices.Equal(eval.Stored.Command, expectedCommand) {
		return Evaluation{
			Name:   name,
			Status: NotRun,
			Note:   fmt.Sprintf("last recorded run used %q, but .proofrun.yml declares %q", eval.Stored.Command, expectedCommand),
		}
	}
	return eval
}
