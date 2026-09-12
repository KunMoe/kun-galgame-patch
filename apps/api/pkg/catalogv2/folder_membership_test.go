package catalogv2

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func holdingListBody(workID int64, folderIDs ...int64) string {
	ids := make([]string, 0, len(folderIDs))
	for _, id := range folderIDs {
		ids = append(ids, `"`+strconv.FormatInt(id, 10)+`"`)
	}
	return `{"object":"list","items":[{"object":"folder_holding","work_id":"` +
		strconv.FormatInt(workID, 10) + `","folder_ids":[` + strings.Join(ids, ",") +
		`]}],"next_cursor":null}`
}

func holderListBody(next string, uids ...int64) string {
	items := make([]string, 0, len(uids))
	for _, uid := range uids {
		items = append(items, `{"object":"folder_holder","owner_uid":"`+
			strconv.FormatInt(uid, 10)+`"}`)
	}
	cursor := "null"
	if next != "" {
		cursor = `"` + next + `"`
	}
	return `{"object":"list","items":[` + strings.Join(items, ",") + `],"next_cursor":` + cursor + `}`
}

func TestMyFolderHoldingsNamesItsWorksAndTakesTheUserToken(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/me/folders/holdings": {holdingListBody(285, 11, 12)},
	}}
	c := New(face.server(t).URL, "nmk_live_key")

	got, err := c.MyFolderHoldings(context.Background(), "user-jwt", []int64{285, 898})
	if err != nil {
		t.Fatalf("MyFolderHoldings: %v", err)
	}
	if len(got) != 1 || got[0].WorkID != 285 || len(got[0].FolderIDs) != 2 {
		t.Fatalf("want one holding of 285 in two folders, got %+v", got)
	}
	call := face.calls()[0]
	if call.Auth != "Bearer user-jwt" {
		t.Errorf("sent %q, want the reader's own token", call.Auth)
	}
	if !strings.Contains(call.Query, "work_ids=285%2C898") {
		t.Errorf("query %q does not name both works — upstream answers 400 without work_ids", call.Query)
	}
}

// 100 is the face's own cap and it 422s above it, so a longer list has to leave
// as more than one request.
func TestMyFolderHoldingsChunksAtTheCap(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/me/folders/holdings": {`{"object":"list","items":[],"next_cursor":null}`},
	}}
	c := New(face.server(t).URL, "k")

	ids := make([]int64, FolderHoldingsMax+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if _, err := c.MyFolderHoldings(context.Background(), "user-jwt", ids); err != nil {
		t.Fatalf("MyFolderHoldings: %v", err)
	}
	if n := len(face.calls()); n != 2 {
		t.Fatalf("%d works over a cap of %d want 2 requests, got %d", len(ids), FolderHoldingsMax, n)
	}
}

// The reverse lookup is the one folder read that must not carry a user token:
// it answers holders of private folders, and the fence is the application key
// plus the operator-granted folder_holders:read scope.
func TestFolderHoldersTakesTheApplicationKeyAndPages(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/folders/holders": {holderListBody("cur_2", 7), holderListBody("", 121089)},
	}}
	c := New(face.server(t).URL, "nmk_live_key")

	got, err := c.FolderHolders(context.Background(), 285)
	if err != nil {
		t.Fatalf("FolderHolders: %v", err)
	}
	if len(got) != 2 || got[0] != 7 || got[1] != 121089 {
		t.Fatalf("want [7 121089] across two pages, got %v", got)
	}
	calls := face.calls()
	if !strings.Contains(calls[0].Auth, "nmk_live_key") {
		t.Errorf("sent %q, want the application key", calls[0].Auth)
	}
	if !strings.Contains(calls[0].Query, "work_id=285") {
		t.Errorf("query %q carries no work_id — it is required", calls[0].Query)
	}
	if !strings.Contains(calls[1].Query, "cursor=cur_2") {
		t.Errorf("second page sent %q, want the cursor the first page handed back", calls[1].Query)
	}
}

func TestUserFoldersReadsTheModerationPath(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/moderation/users/121089/folders": {folderListBody("", 11, 12)},
	}}
	c := New(face.server(t).URL, "nmk_live_key")

	got, err := c.UserFolders(context.Background(), "admin-jwt", 121089)
	if err != nil {
		t.Fatalf("UserFolders: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want two folders, got %d", len(got))
	}
	if auth := face.calls()[0].Auth; auth != "Bearer admin-jwt" {
		t.Errorf("sent %q, want the moderator's own token — an application key cannot moderate", auth)
	}
}
