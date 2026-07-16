package trustclient

import (
	"strconv"
	"testing"
	"time"
)

// The signature gate is the ONLY authentication on the public /trust/callback
// route, so every rejection path must hold (mirrors kungal's coverage).

func TestVerifyCallbackSignature(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"disposition_id":1,"subject_kind":"patch_resource","subject_id":"7","action":1,"reason_code":"spam"}`)
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := SignPayload(secret, ts, body)

	t.Run("valid signature passes", func(t *testing.T) {
		if !VerifyCallbackSignature(secret, ts, sig, body, now) {
			t.Fatal("expected valid signature to verify")
		}
	})

	t.Run("empty secret fails closed", func(t *testing.T) {
		// Unconfigured moyu must reject EVERY callback, even a correctly signed
		// one for an empty secret.
		if VerifyCallbackSignature("", ts, SignPayload("", ts, body), body, now) {
			t.Fatal("empty secret must reject all callbacks")
		}
	})

	t.Run("missing headers fail", func(t *testing.T) {
		if VerifyCallbackSignature(secret, "", sig, body, now) {
			t.Fatal("empty timestamp must fail")
		}
		if VerifyCallbackSignature(secret, ts, "", body, now) {
			t.Fatal("empty signature must fail")
		}
	})

	t.Run("wrong secret fails", func(t *testing.T) {
		if VerifyCallbackSignature("other-secret", ts, sig, body, now) {
			t.Fatal("signature under a different secret must fail")
		}
	})

	t.Run("tampered body fails", func(t *testing.T) {
		tampered := []byte(`{"disposition_id":1,"subject_kind":"patch_resource","subject_id":"8","action":1,"reason_code":"spam"}`)
		if VerifyCallbackSignature(secret, ts, sig, tampered, now) {
			t.Fatal("tampered body must fail")
		}
	})

	t.Run("tampered timestamp fails", func(t *testing.T) {
		otherTS := strconv.FormatInt(now.Unix()-1, 10)
		if VerifyCallbackSignature(secret, otherTS, sig, body, now) {
			t.Fatal("timestamp not covered by the signature must fail")
		}
	})

	t.Run("non-numeric timestamp fails", func(t *testing.T) {
		if VerifyCallbackSignature(secret, "not-a-number", SignPayload(secret, "not-a-number", body), body, now) {
			t.Fatal("non-numeric timestamp must fail")
		}
	})

	t.Run("timestamp window", func(t *testing.T) {
		// Just inside ±CallbackWindow passes; just outside fails (replay guard).
		for _, tc := range []struct {
			name string
			at   time.Time
			want bool
		}{
			{"stale beyond window", now.Add(CallbackWindow + time.Minute), false},
			{"future beyond window", now.Add(-CallbackWindow - time.Minute), false},
			{"within window (old)", now.Add(CallbackWindow - time.Minute), true},
			{"within window (new)", now.Add(-CallbackWindow + time.Minute), true},
		} {
			if got := VerifyCallbackSignature(secret, ts, sig, body, tc.at); got != tc.want {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		}
	})
}
