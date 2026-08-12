package config

import (
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
		if got.Command != check.Command || got.Required != check.Required {
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
