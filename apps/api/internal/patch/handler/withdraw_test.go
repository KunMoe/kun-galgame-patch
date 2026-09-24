package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
	"kun-galgame-patch-api/pkg/config"
)

const withdrawGID = 9000

type withdrawFake struct {
	mu          sync.Mutex
	state       string
	calls       []string
	match       string
	deleteFails bool
}

func (f *withdrawFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v2/catalog/works":
			_, _ = fmt.Fprintf(w, `{"object":"list","items":[{"object":"work","id":"%d"}]}`, withdrawGID)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/me/claims/"):
			f.calls = append(f.calls, "GET")
			w.Header().Set("ETag", fmt.Sprintf(`"c%d.%s"`, withdrawGID, f.state))
			_, _ = fmt.Fprintf(w, `{"object":"claim","id":"%d","state":"%s","last_event":{"id":"5"}}`,
				withdrawGID, f.state)
		case r.Method == http.MethodPatch:
			f.calls = append(f.calls, "PATCH")
			f.match = r.Header.Get("If-Match")
			_, _ = fmt.Fprintf(w, `{"object":"claim","id":"%d","state":"draft"}`, withdrawGID)
		case r.Method == http.MethodDelete:
			f.calls = append(f.calls, "DELETE")
			if f.deleteFails {
				catalogv2test.Problem(w, r, "SERVICE_UNAVAILABLE", "claims are not bound.", nil)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			catalogv2test.Problem(w, r, "NOT_FOUND", "No claim with this id.", nil)
		}
	}
}

func withdraw(t *testing.T, state string) *withdrawFake {
	t.Helper()
	return withdrawWith(t, &withdrawFake{state: state})
}

func withdrawWith(t *testing.T, fake *withdrawFake) *withdrawFake {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	h := New(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil, nil)
	ta := testutil.NewTestApp(t)
	ta.App.Delete("/galgame/:gid", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.WithdrawGalgameSubmission)
	session := ta.CreateTestSession(t, 42)

	resp := ta.Request(t, http.MethodDelete, fmt.Sprintf("/galgame/%d", withdrawGID), "", session)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, testutil.ReadBody(t, resp))
	}
	return fake
}

func TestWithdrawDeletesTheDraftAPendingSubmissionLeavesBehind(t *testing.T) {
	fake := withdraw(t, "pending")
	if got := strings.Join(fake.calls, ","); got != "GET,PATCH,DELETE" {
		t.Fatalf("calls = %s; a withdrawn submission that is not deleted is a draft "+
			"its own author can no longer see", got)
	}
	if fake.match != fmt.Sprintf(`"c%d.pending"`, withdrawGID) {
		t.Errorf("If-Match = %q; with * an approval racing the read would put the "+
			"delete on a work that just went live", fake.match)
	}
}

func TestWithdrawOnlyUnpublishesALiveClaim(t *testing.T) {
	fake := withdraw(t, "live")
	if got := strings.Join(fake.calls, ","); got != "GET,PATCH" {
		t.Fatalf("calls = %s; the work behind a live claim predates this site and "+
			"deleting it takes the catalog entry down with the patch page", got)
	}
}

func TestWithdrawFinishesADraftLeftByAnInterruptedDelete(t *testing.T) {
	fake := withdraw(t, "draft")
	if got := strings.Join(fake.calls, ","); got != "GET,DELETE" {
		t.Fatalf("calls = %s; a draft cannot be withdrawn again, only deleted", got)
	}
}

// The withdraw has landed by the time the delete runs, and the submission is
// already off its author's list; answering 5xx there reported a withdraw that
// had happened as one that had not.
func TestWithdrawThatLandedIsNotUndoneByAFailedDelete(t *testing.T) {
	fake := withdrawWith(t, &withdrawFake{state: "pending", deleteFails: true})
	if got := strings.Join(fake.calls, ","); got != "GET,PATCH,DELETE" {
		t.Fatalf("calls = %s", got)
	}
}
