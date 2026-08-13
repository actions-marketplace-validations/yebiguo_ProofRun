// Package config reads and writes .proofrun.yml, the user-editable list of
// named checks (e.g. "test", "build", "lint") and the argv each one runs.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the config file's name, always resolved relative to the
// directory ProofRun is invoked from.
const FileName = ".proofrun.yml"

// Check is a single named check: the argv to run and whether a failing or
// stale result should be treated as blocking by `status --strict`.
//
// Command is a list, not a single shell-style string: an executed command
// is always a real argv (see internal/runner, which never goes through a
// shell), and comparing that argv against what config declares must be
// exact and unambiguous. A single string would have to be either
// re-tokenized with a shell parser (whose quoting rules ProofRun can't
// assume, since it targets Windows/macOS/Linux) or compared after
// flattening the real argv into a string — and flattening is lossy:
// []string{"go","test","-run","TestCritical","./..."} and
// []string{"go","test","-run","TestCritical ./..."} join to the identical
// string despite being different commands (the second runs zero tests and
// exits 0 in most repos). An argv list sidesteps both problems.
type Check struct {
	Command  []string `yaml:"command"`
	Required bool     `yaml:"required"`
}

// Config is the parsed contents of .proofrun.yml.
type Config struct {
	Checks map[string]Check `yaml:"checks"`
}

// Default returns the starter config written by `proofrun init`.
func Default() *Config {
	return &Config{
		Checks: map[string]Check{
			"test":  {Command: []string{"pytest"}, Required: true},
			"build": {Command: []string{"npm", "run", "build"}, Required: true},
			"lint":  {Command: []string{"ruff", "check", "."}, Required: false},
		},
	}
}

// Path returns the full path to the config file inside dir.
func Path(dir string) string {
	return filepath.Join(dir, FileName)
}

// Exists reports whether dir already has a config file.
func Exists(dir string) bool {
	_, err := os.Stat(Path(dir))
	return err == nil
}

// Load reads and parses the config file in dir. A check declared with an
// empty command (no argv elements, or only whitespace-only elements) is
// rejected outright, rather than silently accepted: downstream, "this
// check is declared in config but has no command to compare against" is
// indistinguishable from "this check isn't declared at all" for the
// purposes of validating that a recorded result actually ran the right
// thing — so an empty command here would let any zero-exit command
// satisfy a `required: true` check. Better to fail loudly at load time
// than let that gap through silently.
func Load(dir string) (*Config, error) {
	data, err := os.ReadFile(Path(dir))
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", FileName, err)
	}
	if cfg.Checks == nil {
		cfg.Checks = map[string]Check{}
	}

	var empty []string
	for name, check := range cfg.Checks {
		if !hasRealCommand(check.Command) {
			empty = append(empty, name)
		}
	}
	if len(empty) > 0 {
		sort.Strings(empty)
		return nil, fmt.Errorf("%s: check(s) with an empty command: %s", FileName, strings.Join(empty, ", "))
	}

	return &cfg, nil
}

func hasRealCommand(command []string) bool {
	for _, token := range command {
		if strings.TrimSpace(token) != "" {
			return true
		}
	}
	return false
}

// Save writes cfg to dir as .proofrun.yml.
func (c *Config) Save(dir string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(dir), data, 0o644)
}
