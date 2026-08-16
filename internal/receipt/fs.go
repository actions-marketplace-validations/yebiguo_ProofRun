package receipt

import (
	"os"
	"path/filepath"

	"github.com/yebiguo/proofrun/internal/git"
)

// ensureDir creates DirName inside dir (if needed) and makes a best-effort
// attempt to keep it out of the project's git history — see
// git.EnsureIgnored. Shared by Save and LoadOrCreateSecret, which both need
// DirName to exist before writing into it.
func ensureDir(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		return err
	}
	git.EnsureIgnored(dir, DirName+"/")
	return nil
}

// writeFileAtomic writes data to a temp file in the same directory as path,
// then renames it into place, instead of writing directly to path.
//
// This matters specifically because DirName's contents can come from a
// checked-out, untrusted repository (that's the whole reason ProofRun's
// GitHub Action clears .proofrun/ before re-running rather than trusting
// whatever's there). os.WriteFile opens path with O_TRUNC and follows
// symlinks — if something had pre-planted path as a symlink pointing
// outside the project (e.g. at an arbitrary file the user happens to have
// write access to), writing "through" it would truncate and overwrite
// whatever it points at. os.Rename, by contrast, replaces the directory
// entry at path — symlink or not — without ever following or writing
// through it, so this is safe against that regardless of what (if
// anything) currently sits at path.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
