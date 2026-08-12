// Package config reads and writes .proofrun.yml, the user-editable list of
// named checks (e.g. "test", "build", "lint") and the shell command each one
// runs.
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

// Check is a single named check: the command to run and whether a failing
// or stale result should be treated as blocking by `status --strict`.
type Check struct {
	Command  string `yaml:"command"`
	Required bool   `yaml:"required"`
}

// Config is the parsed contents of .proofrun.yml.
type Config struct {
	Checks map[string]Check `yaml:"checks"`
}

// Default returns the starter config written by `proofrun init`.
func Default() *Config {
	return &Config{
		Checks: map[string]Check{
			"test":  {Command: "pytest", Required: true},
			"build": {Command: "npm run build", Required: true},
			"lint":  {Command: "ruff check .", Required: false},
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
// empty (or whitespace-only) command is rejected outright, rather than
// silently accepted: downstream, "this check is declared in config but has
// no command to compare against" is indistinguishable from "this check
// isn't declared at all" for the purposes of validating that a recorded
// result actually ran the right thing — so an empty command here would
// let any zero-exit command satisfy a `required: true` check. Better to
// fail loudly at load time than let that gap through silently.
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
		if strings.TrimSpace(check.Command) == "" {
			empty = append(empty, name)
		}
	}
	if len(empty) > 0 {
		sort.Strings(empty)
		return nil, fmt.Errorf("%s: check(s) with an empty command: %s", FileName, strings.Join(empty, ", "))
	}

	return &cfg, nil
}

// Save writes cfg to dir as .proofrun.yml.
func (c *Config) Save(dir string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(dir), data, 0o644)
}
