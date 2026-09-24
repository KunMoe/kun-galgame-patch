package upload

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

func answer(t *testing.T, fn func(fiber.Ctx, error) error, err error) (int, int, string) {
	t.Helper()
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return fn(c, err) })
	res, rerr := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if rerr != nil {
		t.Fatal(rerr)
	}
	raw, _ := io.ReadAll(res.Body)
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if jerr := json.Unmarshal(raw, &body); jerr != nil {
		t.Fatalf("body %q: %v", raw, jerr)
	}
	return res.StatusCode, body.Code, body.Message
}

func artifactErr(kind upstream.Kind, status int, cause error) error {
	return &upstream.Error{Service: "artifact", Kind: kind, Status: status, Cause: cause, Detail: "leaky detail"}
}

func TestUploadErrorAnswers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   int
	}{
		{"rotated credentials are a logged 500, not the reader's 400",
			artifactErr(upstream.Internal, 401, nil), 500, 50000},
		{"an unconfigured client is ours", artifactclient.ErrNotConfigured, 500, 50000},
		{"upload switched off", artifactErr(upstream.Unavailable, 503, artifactclient.ErrUploadDisabled), 503, 50300},
		{"a file too big is the reader's", artifactErr(upstream.Rejected, 413, artifactclient.ErrTooBig), 400, 40000},
		{"the site's daily quota", artifactErr(upstream.RateLimited, 429, artifactclient.ErrQuotaExceeded), 429, 42900},
		{"a store outage", artifactErr(upstream.Unavailable, 500, nil), 503, 50300},
		{"resume after completion", artifactErr(upstream.Conflict, 409, nil), 409, 40900},
		{"someone else's upload", errNotUploadOwner, 403, 40300},
		{"a published file", errArtifactInUse, 409, 40900},
		{"the reader's own quota", errOverDailyUpload, 400, 40000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, code, msg := answer(t, uploadError, tc.err)
			if status != tc.status || code != tc.code {
				t.Errorf("got %d/%d (%q), want %d/%d", status, code, msg, tc.status, tc.code)
			}
			if msg == "leaky detail" {
				t.Error("upstream detail reached the reader")
			}
		})
	}
}

func imageErr(kind upstream.Kind, status int, cause error) error {
	return &upstream.Error{Service: "image", Kind: kind, Status: status, Cause: cause}
}

func TestImageErrorAnswers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"too large", imageErr(upstream.Rejected, 413, imageclient.ErrFileTooLarge), 400},
		{"format denied", imageErr(upstream.Rejected, 400, imageclient.ErrMIMEDenied), 400},
		{"undecodable", imageErr(upstream.Rejected, 400, imageclient.ErrDecodeFailed), 400},
		{"moderation", imageErr(upstream.Rejected, 422, imageclient.ErrModerationRejected), 422},
		{"site quota", imageErr(upstream.RateLimited, 429, imageclient.ErrQuotaExceeded), 429},
		{"preset denied is moyu's allowlist", imageErr(upstream.Internal, 403, nil), 500},
		{"store failure", imageErr(upstream.Unavailable, 500, nil), 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if status, code, msg := answer(t, imageError, tc.err); status != tc.status {
				t.Errorf("got %d/%d (%q), want %d", status, code, msg, tc.status)
			}
		})
	}
}
