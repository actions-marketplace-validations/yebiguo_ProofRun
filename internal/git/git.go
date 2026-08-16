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

// IsTracked reports whether relPath (relative to dir) is tracked by git —
// present in the index, regardless of whether it currently has uncommitted
// modifications. Used to refuse trusting a machine-local secret that
// turns out to actually be checked into the repository: anyone with read
// access to the repo already knows its content, so a tracked file can
// never serve as a local-only secret no matter how it got there (an
// attacker committing one deliberately, or a past mistake before
// EnsureIgnored existed).
//
// Returns false on any error (not a repo, git itself fails, ...) rather
// than propagating it — this is a soft, best-effort signal like
// EnsureIgnored, not something callers should treat as authoritative in
// the way IsRepo is.
func IsTracked(dir, relPath string) bool {
	_, err := run(dir, "ls-files", "--error-unmatch", "--", relPath)
	return err == nil
}

// EnsureIgnored makes a best-effort attempt to keep relPath (e.g.
// ".proofrun/") out of dir's git status/diff, via the repository's local
// info/exclude file — never dir's own .gitignore, which ProofRun has no
// business editing on a consumer project's behalf. info/exclude lives
// inside .git (resolved with `git rev-parse --git-path`, so this works
// correctly for worktrees too, not just plain repos) and is never
// committed, which is exactly the property needed here: `proofrun init`
// doesn't touch a project's .gitignore, so a fresh checkout that never
// bothered to add its own ".proofrun/" entry would otherwise let a plain
// `git add .` pick up .proofrun/ wholesale — including, as of v0.3, the
// local receipt-signing key living inside it.
//
// This is deliberately silent on any failure (not a git repo, info/exclude
// unresolvable or unwritable, ...): it's defense in depth on top of
// whatever ignore rules the project already has, not something proofrun's
// core functionality should ever depend on succeeding.
func EnsureIgnored(dir, relPath string) {
	if !IsRepo(dir) {
		return
	}
	out, err := run(dir, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return
	}
	excludePath := strings.TrimSpace(out)
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(dir, excludePath)
	}

	existing, _ := os.ReadFile(excludePath)
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == relPath {
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		fmt.Fprintln(f)
	}
	fmt.Fprintln(f, relPath)
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
