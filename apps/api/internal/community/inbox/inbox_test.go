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

func TestPlanMirrorsFollowedWithEmptyAnchor(t *testing.T) {
	actor := int64(5)
	in := New(nil, anchor.New(nil, failingOwner{}), nil)
	in.localIDs = func(ids []int64) (map[int64]struct{}, error) {
		return map[int64]struct{}{3: {}, 5: {}}, nil
	}
	rows, err := in.plan(context.Background(), []communityclient.NotificationView{{
		ID: 11, UserID: 3, Kind: communityclient.NotificationKindFollowed,
		ActorID: &actor, ActorCount: 1, ThreadID: 0, AnchorKind: 0, AnchorID: "",
	}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Type != "follow" || row.Content != "关注了您!" || row.Link != "/user/5/resource" {
		t.Errorf("row = %+v", row)
	}
	if row.RecipientID != 3 || row.SenderID == nil || *row.SenderID != 5 {
		t.Errorf("ids recipient=%d sender=%v", row.RecipientID, row.SenderID)
	}
	if row.ThreadID != nil {
		t.Errorf("thread_id = %v", row.ThreadID)
	}
	if row.NotificationID != 11 {
		t.Errorf("notification_id = %d", row.NotificationID)
	}
}

func TestPlanFollowFoldNamesTheCount(t *testing.T) {
	actor := int64(5)
	in := New(nil, nil, nil)
	in.localIDs = func([]int64) (map[int64]struct{}, error) {
		return map[int64]struct{}{3: {}, 5: {}}, nil
	}
	rows, err := in.plan(context.Background(), []communityclient.NotificationView{{
		ID: 11, UserID: 3, Kind: communityclient.NotificationKindFollowed,
		ActorID: &actor, ActorCount: 4, AnchorKind: 0, AnchorID: "",
	}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(rows) != 1 || rows[0].Content != "等 4 人关注了您!" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestPlanDoesNotMirrorFolloweeThreadCreated(t *testing.T) {
	in := New(nil, nil, nil)
	in.localIDs = func([]int64) (map[int64]struct{}, error) {
		t.Fatal("kind 9 looked up local users")
		return nil, nil
	}
	rows, err := in.plan(context.Background(), []communityclient.NotificationView{{
		ID: 11, UserID: 3, Kind: communityclient.NotificationKindFolloweeThreadCreated,
		AnchorKind: communityclient.AnchorBoard, AnchorID: "1",
	}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestPlanSkipsFollowedWhenRecipientIsNotLocal(t *testing.T) {
	actor := int64(5)
	in := New(nil, nil, nil)
	in.localIDs = func([]int64) (map[int64]struct{}, error) {
		return map[int64]struct{}{5: {}}, nil
	}
	rows, err := in.plan(context.Background(), []communityclient.NotificationView{{
		ID: 11, UserID: 3, Kind: communityclient.NotificationKindFollowed,
		ActorID: &actor, ActorCount: 1,
	}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestPlanMirrorsFolloweeActivityAndRetractsAnEmptyFold(t *testing.T) {
	actor := int64(5)
	in := New(nil, nil, nil)
	in.localIDs = func([]int64) (map[int64]struct{}, error) {
		return map[int64]struct{}{3: {}, 5: {}}, nil
	}
	rows, err := in.plan(context.Background(), []communityclient.NotificationView{
		{
			ID: 21, UserID: 3, Kind: communityclient.NotificationKindFolloweeActivity, Seq: 40,
			ActorID: &actor, ActorCount: 1, ItemCount: 2,
			Activity: &communityclient.NotificationActivity{ObjectLabel: "Galgame 补丁", Title: "T", URL: "https://www.moyu.moe/resource/9"},
		},
		{ID: 22, UserID: 3, Kind: communityclient.NotificationKindFolloweeActivity, Seq: 41, ActorID: &actor},
		{ID: 23, UserID: 4, Kind: communityclient.NotificationKindFolloweeActivity, Seq: 42, ActorID: &actor, ItemCount: 1},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the live fold and the retraction (recipient 4 is not local)", rows)
	}
	live, gone := rows[0], rows[1]
	if live.Retract || live.Type != "followActivity" || live.Link != "/resource/9" || live.Seq != 40 ||
		live.ThreadID != nil || live.SenderID == nil || *live.SenderID != 5 {
		t.Errorf("live = %+v", live)
	}
	if !gone.Retract || gone.NotificationID != 22 || gone.Seq != 41 {
		t.Errorf("retraction = %+v", gone)
	}
}
