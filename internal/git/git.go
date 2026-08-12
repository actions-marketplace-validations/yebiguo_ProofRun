// Package git computes the two fingerprints ProofRun binds a check result
// to: the current commit (HEAD) and a hash of everything not yet committed
// (staged changes, unstaged changes to tracked files, and untracked files
// that git would track if added). Any byte of difference in either of
// those must change the fingerprint — that guarantee is the entire product.
package git

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoCommits is returned when the repository has no commits yet, so
// there is no HEAD to bind a fingerprint to.
var ErrNoCommits = errors.New("no commits yet: proofrun requires at least one git commit")

// ErrNotARepo is returned when dir is not inside a git working tree.
var ErrNotARepo = errors.New("not a git repository")

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// IsRepo reports whether dir is inside a git working tree.
func IsRepo(dir string) bool {
	out, err := run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// Head returns the full SHA of the current commit.
func Head(dir string) (string, error) {
	if !IsRepo(dir) {
		return "", ErrNotARepo
	}
	out, err := run(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", ErrNoCommits
	}
	return strings.TrimSpace(out), nil
}

// DiffFingerprint returns a sha256 hex digest covering every uncommitted
// change in the working tree relative to HEAD: staged changes, unstaged
// changes to tracked files, and untracked-but-not-ignored files. It does
// not use any whitespace-ignoring diff options, so a single changed space
// in a tracked file produces a different fingerprint.
//
// excludeTopLevelDirs names top-level, repo-relative directories (e.g.
// ".proofrun") whose untracked contents are never folded into the
// fingerprint. This exists so ProofRun's own local state directory — which
// only comes into being *because* a check was run — cannot invalidate the
// very fingerprint that check was just bound to.
func DiffFingerprint(dir string, excludeTopLevelDirs ...string) (string, error) {
	if _, err := Head(dir); err != nil {
		return "", err
	}

	diff, err := run(dir, "diff", "HEAD", "--no-color")
	if err != nil {
		return "", err
	}

	untracked, err := run(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}

	excluded := make(map[string]bool, len(excludeTopLevelDirs))
	for _, d := range excludeTopLevelDirs {
		excluded[filepath.ToSlash(d)] = true
	}

	var files []string
	for _, f := range strings.Split(untracked, "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if top := strings.SplitN(f, "/", 2)[0]; excluded[top] {
			continue
		}
		files = append(files, f)
	}
	sort.Strings(files)

	h := sha256.New()
	h.Write([]byte(diff))
	for _, f := range files {
		content, readErr := os.ReadFile(filepath.Join(dir, f))
		if readErr != nil {
			// The file was listed but is unreadable (e.g. removed or
			// permission-denied between the listing and the read). Fold
			// that fact into the hash rather than silently skipping the
			// file, so the fingerprint still changes instead of matching
			// a state it never actually observed.
			content = []byte(fmt.Sprintf("<<unreadable: %v>>", readErr))
		}
		fmt.Fprintf(h, "\x00untracked-file:%s\x00", f)
		h.Write(content)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
