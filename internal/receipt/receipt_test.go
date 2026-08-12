package receipt

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_MissingReceiptReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Checks) != 0 {
		t.Fatalf("expected no checks for a missing receipt, got %d", len(r.Checks))
	}
	if r.Schema != SchemaVersion {
		t.Fatalf("Schema = %q, want %q", r.Schema, SchemaVersion)
	}
}

func TestSaveThenLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	r := New()
	fp := Fingerprint{Head: "abc123", DiffSHA256: "deadbeef"}
	cr := NewResult([]string{"pytest"}, 0, time.Now(), 1234*time.Millisecond, fp)
	r.Set("test", cr)

	if err := r.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := loaded.Checks["test"]
	if !ok {
		t.Fatal("loaded receipt is missing the \"test\" check")
	}
	if got.Status != StatusPass {
		t.Fatalf("Status = %q, want %q", got.Status, StatusPass)
	}
	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if got.DurationMS != 1234 {
		t.Fatalf("DurationMS = %d, want 1234", got.DurationMS)
	}
	if !got.VerifiedAgainst.Equal(fp) {
		t.Fatalf("VerifiedAgainst = %+v, want %+v", got.VerifiedAgainst, fp)
	}
}

func TestNewResult_ExitCodeDeterminesStatus(t *testing.T) {
	fp := Fingerprint{Head: "h", DiffSHA256: "d"}

	pass := NewResult([]string{"x"}, 0, time.Now(), 0, fp)
	if pass.Status != StatusPass {
		t.Fatalf("exit code 0 => Status = %q, want %q", pass.Status, StatusPass)
	}

	for _, code := range []int{1, 2, -1, 127} {
		fail := NewResult([]string{"x"}, code, time.Now(), 0, fp)
		if fail.Status != StatusFail {
			t.Fatalf("exit code %d => Status = %q, want %q", code, fail.Status, StatusFail)
		}
	}
}

func TestSave_CreatesProofrunDir(t *testing.T) {
	dir := t.TempDir()
	r := New()
	if err := r.Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, DirName)); err != nil {
		t.Fatalf("%s directory was not created: %v", DirName, err)
	}
}

func TestLoad_RejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil, want an error for malformed JSON")
	}
}

func TestSet_OverwritesExistingCheck(t *testing.T) {
	r := New()
	fp := Fingerprint{Head: "h1", DiffSHA256: "d1"}
	r.Set("test", NewResult([]string{"x"}, 1, time.Now(), 0, fp))
	if r.Checks["test"].Status != StatusFail {
		t.Fatal("expected initial status to be fail")
	}

	fp2 := Fingerprint{Head: "h2", DiffSHA256: "d2"}
	r.Set("test", NewResult([]string{"x"}, 0, time.Now(), 0, fp2))
	if r.Checks["test"].Status != StatusPass {
		t.Fatal("Set() did not overwrite the existing check result")
	}
	if r.Checks["test"].VerifiedAgainst != fp2 {
		t.Fatal("Set() did not overwrite the fingerprint")
	}
}
