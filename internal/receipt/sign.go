package receipt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// computeSignature returns the hex-encoded HMAC-SHA256 of cr's fields,
// under key, with cr.Signature itself blanked first so the signature never
// signs itself.
//
// Signing the JSON encoding of the whole struct — not a hand-rolled
// concatenation of fields — is deliberate: encoding/json marshals a given
// struct's fields in declaration order, deterministically, as long as the
// struct definition doesn't change. That's exactly the same assumption
// every other part of this package already makes about CheckResult's
// shape, so it costs nothing new to rely on here too.
func computeSignature(key []byte, cr CheckResult) string {
	cr.Signature = ""
	data, err := json.Marshal(cr)
	if err != nil {
		// CheckResult's fields (strings, ints, a time.Time, a slice of
		// strings) are all trivially JSON-marshalable; this cannot
		// actually fail. If it ever does, something is deeply wrong
		// enough that panicking here beats silently signing empty bytes.
		panic("receipt: CheckResult failed to marshal for signing: " + err.Error())
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifySignature reports whether cr.Signature is a valid HMAC-SHA256 over
// its other fields under key.
//
// An empty Signature always fails — there is no grandfather clause for
// receipts written before v0.3, and no distinction between "never signed"
// and "signed with the wrong key/content"; both mean the same thing here:
// this stored result cannot be trusted as evidence. See the package doc's
// "tamper-evident, not tamper-proof" framing (secret.go) for what this is
// and isn't meant to catch — a wrong signature reads as NOT RUN via the
// same path an entirely missing stored result already takes, not a new
// status.
func verifySignature(key []byte, cr CheckResult) bool {
	if cr.Signature == "" {
		return false
	}
	want := computeSignature(key, cr)
	// Constant-time comparison is the standard, correct idiom for MAC
	// verification. The realistic exposure here (a local, single-user CLI)
	// makes a timing side-channel a non-issue in practice, but there's no
	// reason to reach for the string-equality operator when the stdlib
	// gives you the right tool for free.
	return hmac.Equal([]byte(cr.Signature), []byte(want))
}
