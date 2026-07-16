package trustclient

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// CallbackWindow is the ±window an enforcement-callback timestamp may fall
// within (matches the infra trust/community convention).
const CallbackWindow = 5 * time.Minute

// SignPayload mirrors the trust service's SignPayload:
// hex(HMAC-SHA256(secret, timestamp + "." + body)). Exposed so callers verify
// identically against the RAW request body bytes.
func SignPayload(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCallbackSignature checks the X-Trust-Signature HMAC on an inbound
// enforcement callback: the signature must equal SignPayload(secret, timestamp,
// body) and the timestamp must fall within ±CallbackWindow of now. An empty
// secret (unconfigured) fails CLOSED — every callback is rejected — so a
// misconfigured moyu never applies an unauthenticated enforcement action.
func VerifyCallbackSignature(secret, timestamp, signature string, body []byte, now time.Time) bool {
	if secret == "" || timestamp == "" || signature == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	skew := now.Sub(time.Unix(ts, 0))
	if skew < -CallbackWindow || skew > CallbackWindow {
		return false
	}
	expected := SignPayload(secret, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}
