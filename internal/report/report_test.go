package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yebiguo/proofrun/internal/receipt"
)

func TestBuild_NotRunHasNoExecutionFields(t *testing.T) {
	evals := []receipt.Evaluation{{Name: "test", Status: receipt.NotRun}}
	r := Build(evals, map[string]bool{"test": true})

	if len(r.Checks) != 1 {
		t.Fatalf("got %d checks, want 1", len(r.Checks))
	}
	c := r.Checks[0]
	if c.Status != "NOT RUN" {
		t.Fatalf("Status = %q, want %q", c.Status, "NOT RUN")
	}
	if c.ExitCode != nil {
		t.Fatal("ExitCode should be nil for a check that never ran")
	}
	if !c.Required {
		t.Fatal("Required should reflect the config lookup")
	}
}

func TestBuild_PassHasExecutionFields(t *testing.T) {
	stored := &receipt.CheckResult{
		Status:     receipt.StatusPass,
		Command:    []string{"pytest"},
		ExitCode:   0,
		DurationMS: 1500,
		StartedAt:  time.Now(),
	}
	evals := []receipt.Evaluation{{Name: "test", Status: receipt.Pass, Stored: stored}}
	r := Build(evals, map[string]bool{})

	c := r.Checks[0]
	if c.ExitCode == nil || *c.ExitCode != 0 {
		t.Fatal("ExitCode should be populated for a PASS")
	}
	if c.DurationMS == nil || *c.DurationMS != 1500 {
		t.Fatal("DurationMS should be populated for a PASS")
	}
}

func TestReport_JSONRoundTrips(t *testing.T) {
	evals := []receipt.Evaluation{{Name: "test", Status: receipt.Pass, Stored: &receipt.CheckResult{ExitCode: 0}}}
	r := Build(evals, nil)

	data, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded["schema"] != receipt.SchemaVersion {
		t.Fatalf("schema = %v, want %v", decoded["schema"], receipt.SchemaVersion)
	}
}

func TestReport_TerminalContainsAllCheckNames(t *testing.T) {
	evals := []receipt.Evaluation{
		{Name: "build", Status: receipt.NotRun},
		{Name: "test", Status: receipt.Pass, Stored: &receipt.CheckResult{Command: []string{"pytest"}}},
	}
	r := Build(evals, map[string]bool{"build": true})

	out := r.Terminal()
	if !strings.Contains(out, "build") || !strings.Contains(out, "NOT RUN") {
		t.Fatalf("terminal output missing build/NOT RUN:\n%s", out)
	}
	if !strings.Contains(out, "test") || !strings.Contains(out, "PASS") {
		t.Fatalf("terminal output missing test/PASS:\n%s", out)
	}
}

func TestReport_TerminalEmptyCase(t *testing.T) {
	r := Build(nil, nil)
	out := r.Terminal()
	if !strings.Contains(out, "no checks found") {
		t.Fatalf("expected guidance message for empty report, got:\n%s", out)
	}
}
