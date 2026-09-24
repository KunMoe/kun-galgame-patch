package inbox

import (
	"context"
	"errors"
	"testing"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"
)

func TestNilInboxIsSafe(t *testing.T) {
	var in *Inbox
	if in.Configured() {
		t.Fatal("nil Inbox is configured")
	}
	n, err := in.Sync(context.Background())
	if n != 0 || err != nil {
		t.Errorf("Sync = %d, %v", n, err)
	}
	in.MarkThreadRead(1, 7, 12)
	in.ForwardRead(context.Background(), 1, []int64{11})
}

type failingOwner struct{}

func (failingOwner) ResourcePatchIDs([]int) (map[int]int, error) {
	return nil, errors.New("connection refused")
}

// A resource wall whose game could not be read resolved to game 0, which plan
// takes for a deleted resource: the notification was skipped and the cursor
// written past it in the same transaction, so it was lost for good. The page
// has to fail before that transaction — which is why db is nil here: reaching
// it would panic.
func TestAFailedAnchorLookupFailsThePageBeforeTheCursorMoves(t *testing.T) {
	in := New(nil, anchor.New(nil, failingOwner{}), nil)
	_, err := in.applyPage(context.Background(), []communityclient.NotificationView{{
		ID: 11, UserID: 3, Kind: communityclient.NotificationKindReplied, ThreadID: 7,
		AnchorKind: communityclient.AnchorSiteResource, AnchorID: "678",
	}}, 12)
	if err == nil {
		t.Fatal("the page applied without its resource wall")
	}
}
