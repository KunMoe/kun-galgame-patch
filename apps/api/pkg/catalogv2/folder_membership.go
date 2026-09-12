package catalogv2

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Mirrors the catalog's own cap on the batch membership face. Repeated here so
// a caller chunks before spending a request that would come back 422.
const FolderHoldingsMax = 100

type FolderHolding struct {
	WorkID    int64
	FolderIDs []int64
}

type folderHoldingWire struct {
	WorkID    string   `json:"work_id"`
	FolderIDs []string `json:"folder_ids"`
}

type folderHolderWire struct {
	OwnerUID string `json:"owner_uid"`
}

// MyFolderHoldings answers, for many works at once, which of the reader's own
// folders hold each one. A work the reader holds nowhere is left out of the
// answer entirely rather than returned with an empty list, so absence from the
// map is the negative.
//
// This is the batch form of MyFoldersHolding. Prefer it wherever a page asks
// about more than one game: the single-work face takes one round trip per row.
func (c *Client) MyFolderHoldings(ctx context.Context, accessToken string, workIDs []int64) ([]FolderHolding, error) {
	out := make([]FolderHolding, 0, len(workIDs))
	for _, chunk := range chunkIDs(workIDs, FolderHoldingsMax) {
		q := url.Values{"work_ids": {joinIDs(chunk)}}
		var page List[folderHoldingWire]
		if _, err := c.userDo(ctx, http.MethodGet,
			"/v2/me/folders/holdings?"+q.Encode(), accessToken, nil, &page); err != nil {
			return nil, err
		}
		for _, row := range page.Items {
			out = append(out, row.view())
		}
	}
	return out, nil
}

// FolderHolders answers who keeps this work in a folder, of every visibility:
// a private folder is still a person waiting to hear about the game, which is
// why the fence is the credential rather than the folder's own flag.
//
// It takes the application key and refuses a reader's token, and the key needs
// folder_holders:read on top of catalog:read. That scope is granted by an
// operator, so a fresh deployment whose key has not been granted it gets 403
// SCOPE_REQUIRED here and nowhere else.
func (c *Client) FolderHolders(ctx context.Context, workID int64) ([]int64, error) {
	extra := url.Values{"work_id": {strconv.FormatInt(workID, 10)}}
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderHolderWire], error) {
		var page List[folderHolderWire]
		e := c.get(ctx, "/v2/folders/holders"+folderQuery(cursor, extra), &page)
		return page, e
	})
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		if uid := parseIDOrZero(r.OwnerUID); uid > 0 {
			out = append(out, uid)
		}
	}
	return out, nil
}

// UserFolders is the moderator's read of somebody else's shelf — the preview of
// the purge on the same path. Items are not listed; each folder's ItemCount is
// what a confirmation needs.
func (c *Client) UserFolders(ctx context.Context, accessToken string, ownerUID int64) ([]Folder, error) {
	path := "/v2/moderation/users/" + strconv.FormatInt(ownerUID, 10) + "/folders"
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderWire], error) {
		var page List[folderWire]
		_, e := c.userDo(ctx, http.MethodGet, path+folderQuery(cursor, nil), accessToken, nil, &page)
		return page, e
	})
	return foldersView(rows), err
}

func (h folderHoldingWire) view() FolderHolding {
	ids := make([]int64, 0, len(h.FolderIDs))
	for _, raw := range h.FolderIDs {
		if id := parseIDOrZero(raw); id > 0 {
			ids = append(ids, id)
		}
	}
	return FolderHolding{WorkID: parseIDOrZero(h.WorkID), FolderIDs: ids}
}
