package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestExists_False(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir) {
		t.Fatal("Exists() = true for a directory with no config")
	}
}

func TestSaveThenLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()

	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
	if !Exists(dir) {
		t.Fatal("Exists() = false after Save()")
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Checks) != len(cfg.Checks) {
		t.Fatalf("loaded %d checks, want %d", len(loaded.Checks), len(cfg.Checks))
	}
	for name, check := range cfg.Checks {
		got, ok := loaded.Checks[name]
		if !ok {
			t.Fatalf("loaded config missing check %q", name)
		}
		if !slices.Equal(got.Command, check.Command) || got.Required != check.Required {
			t.Fatalf("check %q = %+v, want %+v", name, got, check)
		}
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want an error for a missing file")
	}
}

func TestLoad_EmptyChecksSection(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{}
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Checks == nil {
		t.Fatal("Load() left Checks nil for an empty checks section")
	}
}

// TestLoad_RejectsEmptyCommand guards against a real forged-PASS hole:
// without this rejection, a check declared with an empty command and
// `required: true` is indistinguishable downstream from "not declared at
// all", which lets any zero-exit command satisfy it under
// `status --strict`. See internal/receipt.EvaluateAgainstCommand, which
// relies on Load() having already ruled this out.
func TestLoad_RejectsEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	yaml := "checks:\n  test:\n    command: []\n    required: true\n"
	if err := os.WriteFile(Path(dir), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want an error for a check with an empty command list")
	}
}

func TestLoad_RejectsMissingCommand(t *testing.T) {
	dir := t.TempDir()
	yaml := "checks:\n  test:\n    required: true\n"
	if err := os.WriteFile(Path(dir), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want an error for a check with no command key at all")
	}
}

func TestLoad_RejectsWhitespaceOnlyCommand(t *testing.T) {
	dir := t.TempDir()
	yaml := "checks:\n  test:\n    command: [\"   \"]\n    required: true\n"
	if err := os.WriteFile(Path(dir), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want an error for a whitespace-only command element")
	}
}

// TestLoad_RejectsOldStringFormat documents an intentional breaking
// change: command used to be a single shell-style string, which made
// argv-identity comparisons lossy (see Check's doc comment). Pre-1.0,
// private repo, no external consumers yet — this is the right time to fix
// the schema, not patch around it.
func TestLoad_RejectsOldStringFormat(t *testing.T) {
	dir := t.TempDir()
	yaml := "checks:\n  test:\n    command: pytest\n    required: true\n"
	if err := os.WriteFile(Path(dir), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want a YAML type error for the old single-string command format")
	}
}

func TestLoad_AcceptsNonEmptyCommands(t *testing.T) {
	dir := t.TempDir()
	if err := Default().Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("Load() unexpectedly failed on a valid default config: %v", err)
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	if got, want := Path(dir), filepath.Join(dir, FileName); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}
