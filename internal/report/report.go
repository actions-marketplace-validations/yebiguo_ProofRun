// Package report formats a set of check evaluations for humans (terminal)
// or machines (JSON). It does no evaluation itself — it only renders
// results the receipt package already computed.
package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yebiguo/proofrun/internal/receipt"
)

// Check is one check's evaluated status, shaped for serialization.
type Check struct {
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Required   bool       `json:"required"`
	Note       string     `json:"note,omitempty"`
	Command    []string   `json:"command,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	DurationMS *int64     `json:"duration_ms,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
}

// Report is the full set of evaluated checks.
type Report struct {
	Schema      string    `json:"schema"`
	GeneratedAt time.Time `json:"generated_at"`
	Checks      []Check   `json:"checks"`
}

// Build converts evaluations (already computed by receipt.Evaluate) plus
// the required-flag lookup from config into a Report.
func Build(evals []receipt.Evaluation, required map[string]bool) Report {
	checks := make([]Check, 0, len(evals))
	for _, e := range evals {
		c := Check{
			Name:     e.Name,
			Status:   string(e.Status),
			Required: required[e.Name],
			Note:     e.Note,
		}
		if e.Stored != nil {
			c.Command = e.Stored.Command
			exitCode := e.Stored.ExitCode
			c.ExitCode = &exitCode
			duration := e.Stored.DurationMS
			c.DurationMS = &duration
			started := e.Stored.StartedAt
			c.StartedAt = &started
		}
		checks = append(checks, c)
	}
	return Report{Schema: receipt.SchemaVersion, GeneratedAt: time.Now().UTC(), Checks: checks}
}

// JSON renders the report as indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Terminal renders the report as a human-readable table plus a summary
// line, e.g. "3 checks: 1 PASS, 1 STALE, 1 NOT RUN".
func (r Report) Terminal() string {
	var b strings.Builder
	counts := map[string]int{}

	for _, c := range r.Checks {
		counts[c.Status]++
		req := ""
		if c.Required {
			req = " (required)"
		}
		fmt.Fprintf(&b, "%-20s %-8s%s\n", c.Name, c.Status, req)
		if c.Note != "" {
			fmt.Fprintf(&b, "%-20s   %s\n", "", c.Note)
		}
		if c.ExitCode != nil {
			fmt.Fprintf(&b, "%-20s   command: %s | exit: %d | duration: %dms | ran at: %s\n",
				"", strings.Join(c.Command, " "), *c.ExitCode, *c.DurationMS, c.StartedAt.Format(time.RFC3339))
		}
	}

	if len(r.Checks) == 0 {
		return "no checks found — run `proofrun init` and `proofrun run <name> -- <cmd>`\n"
	}

	fmt.Fprintf(&b, "\n%d checks:", len(r.Checks))
	for _, status := range []string{"PASS", "FAIL", "STALE", "NOT RUN"} {
		if n := counts[status]; n > 0 {
			fmt.Fprintf(&b, " %d %s,", n, status)
		}
	}
	out := b.String()
	return strings.TrimSuffix(out, ",") + "\n"
}
