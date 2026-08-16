package receipt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeSignature_DeterministicForSameInput(t *testing.T) {
	key := []byte("a-fixed-32-byte-test-key-------")
	cr := CheckResult{Status: StatusPass, Command: []string{"go", "test"}, ExitCode: 0, DurationMS: 100}

	a := computeSignature(key, cr)
	b := computeSignature(key, cr)
	if a != b {
		t.Fatalf("computeSignature is not deterministic: %q vs %q", a, b)
	}
}

func TestComputeSignature_DifferentContentDifferentSignature(t *testing.T) {
	key := []byte("a-fixed-32-byte-test-key-------")
	pass := CheckResult{Status: StatusPass, ExitCode: 0}
	fail := CheckResult{Status: StatusFail, ExitCode: 1}

	if computeSignature(key, pass) == computeSignature(key, fail) {
		t.Fatal("different CheckResult content produced the same signature")
	}
}

func TestComputeSignature_DifferentKeyDifferentSignature(t *testing.T) {
	cr := CheckResult{Status: StatusPass, ExitCode: 0}
	sig1 := computeSignature([]byte("key-one-------------------------"), cr)
	sig2 := computeSignature([]byte("key-two-------------------------"), cr)
	if sig1 == sig2 {
		t.Fatal("different keys produced the same signature for identical content")
	}
}

func TestVerifySignature_EmptySignatureAlwaysFails(t *testing.T) {
	key := []byte("a-fixed-32-byte-test-key-------")
	cr := CheckResult{Status: StatusPass, ExitCode: 0, Signature: ""}
	if verifySignature(key, cr) {
		t.Fatal("an empty Signature verified successfully — it must always fail")
	}
}

func TestVerifySignature_ValidSignatureVerifies(t *testing.T) {
	key := []byte("a-fixed-32-byte-test-key-------")
	cr := CheckResult{Status: StatusPass, ExitCode: 0}
	cr.Signature = computeSignature(key, cr)
	if !verifySignature(key, cr) {
		t.Fatal("a correctly computed signature failed to verify")
	}
}

func TestVerifySignature_TamperedFieldFailsVerification(t *testing.T) {
	key := []byte("a-fixed-32-byte-test-key-------")
	cr := CheckResult{Status: StatusFail, ExitCode: 1}
	cr.Signature = computeSignature(key, cr)

	// Tamper with the signed content after signing — exactly what a
	// hand-edited receipt.json looks like: the fingerprint might even
	// still match, but the signature no longer does.
	cr.Status = StatusPass
	cr.ExitCode = 0

	if verifySignature(key, cr) {
		t.Fatal("verifySignature accepted a CheckResult whose content was changed after signing")
	}
}

// TestLoad_DropsHandEditedEntryEvenWithMatchingFingerprint is the direct
// unit-level version of the adversarial scenario PR #5 proved against the
// GitHub Action's rm-rf-and-rerun defense: a receipt.json hand-edited to
// claim PASS, with a fingerprint that matches perfectly. Day 1's key
// management existed specifically to make this detectable on the pure
// local path (no Action, no rm -rf, just `proofrun status` reading the
// file directly) — this confirms it actually is.
func TestLoad_DropsHandEditedEntryEvenWithMatchingFingerprint(t *testing.T) {
	dir := t.TempDir()
	fp := Fingerprint{Head: "abc123", DiffSHA256: "deadbeef"}

	r := New()
	r.Set("test", NewResult([]string{"pytest"}, 1, time.Now(), time.Second, fp)) // a real FAIL
	if err := r.Save(dir); err != nil {
		t.Fatal(err)
	}

	// Hand-edit the file exactly the way a human (or an unsophisticated
	// agent) would: load the JSON, flip the fields that matter, write it
	// back — without touching the signature, since an attacker who doesn't
	// know this mechanism exists wouldn't think to.
	raw, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk Receipt
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	cr := onDisk.Checks["test"]
	cr.Status = StatusPass
	cr.ExitCode = 0
	onDisk.Checks["test"] = cr
	tampered, err := json.MarshalIndent(&onDisk, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), tampered, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Checks["test"]; ok {
		t.Fatal("Load kept a hand-edited entry whose signature no longer matches its content")
	}

	eval := Evaluate(loaded, "test", fp)
	if eval.Status != NotRun {
		t.Fatalf("Evaluate status = %v, want NotRun — a hand-edited receipt must never read as PASS even with a matching fingerprint", eval.Status)
	}
}

// TestLoad_DropsPreV03UnsignedEntry confirms there is no grandfather clause
// for receipts written before signing existed — matching the locked-in
// design decision to treat "never signed" and "signed wrong" identically,
// both reading as NOT RUN.
func TestLoad_DropsPreV03UnsignedEntry(t *testing.T) {
	dir := t.TempDir()

	oldStyle := Receipt{
		Schema: "proofrun/v1",
		Checks: map[string]CheckResult{
			"test": {
				Status:          StatusPass,
				Command:         []string{"pytest"},
				ExitCode:        0,
				VerifiedAgainst: Fingerprint{Head: "abc123", DiffSHA256: "deadbeef"},
				// no Signature field in this JSON at all
			},
		},
	}
	data, err := json.MarshalIndent(&oldStyle, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Checks["test"]; ok {
		t.Fatal("Load kept a pre-v0.3 entry with no signature at all")
	}
}
