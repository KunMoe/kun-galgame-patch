package catalogv2

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Mirrors the catalog's own caps. Both are refused upstream with 422; they are
// repeated so this site can say so in its own words before spending a request.
const (
	FoldersPerUserMax = 200
	FolderItemsMax    = 10_000
)

const (
	FolderVisibilityPrivate = "private"
	FolderVisibilityPublic  = "public"
)

const folderPageMax = 100

// A whole list is read rather than paged through to the caller: the catalog's
// keyset is an incremental-sync watermark ordered by updated_at, while this
// site's faces are page-numbered and ordered for a reader. The shapes are
// small — production p50 is 5 folders per person and 2 items per folder, p99
// is 370 items — so sorting locally costs one request in the ordinary case.
const folderWalkPages = FolderItemsMax/folderPageMax + 1

type Folder struct {
	ID          int64  `json:"id"`
	OwnerUID    int64  `json:"owner_uid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	IsDefault   bool   `json:"is_default"`
	ItemCount   int    `json:"item_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type FolderItem struct {
	FolderID  int64  `json:"folder_id"`
	WorkID    int64  `json:"work_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type folderWire struct {
	ID          string `json:"id"`
	OwnerUID    string `json:"owner_uid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	IsDefault   bool   `json:"is_default"`
	ItemCount   int    `json:"item_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type folderItemWire struct {
	FolderID  string `json:"folder_id"`
	WorkID    string `json:"work_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (f folderWire) view() Folder {
	return Folder{
		ID: parseIDOrZero(f.ID), OwnerUID: parseIDOrZero(f.OwnerUID), Name: f.Name,
		Description: f.Description, Visibility: f.Visibility, IsDefault: f.IsDefault,
		ItemCount: f.ItemCount, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

func (i folderItemWire) view() FolderItem {
	return FolderItem{
		FolderID: parseIDOrZero(i.FolderID), WorkID: parseIDOrZero(i.WorkID),
		CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt,
	}
}

type FolderWrite struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
	IsDefault   *bool   `json:"is_default,omitempty"`
}

func walkFolderPages[T any](ctx context.Context, fetch func(cursor string) (List[T], error)) ([]T, error) {
	var out []T
	cursor := ""
	for page := 0; page < folderWalkPages; page++ {
		got, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		out = append(out, got.Items...)
		if got.NextCursor == nil || *got.NextCursor == "" || len(got.Items) == 0 {
			return out, nil
		}
		cursor = *got.NextCursor
	}
	return out, nil
}

func folderQuery(cursor string, extra url.Values) string {
	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	q.Set("limit", strconv.Itoa(folderPageMax))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return "?" + q.Encode()
}

func (c *Client) MyFolders(ctx context.Context, accessToken string) ([]Folder, error) {
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderWire], error) {
		var page List[folderWire]
		_, e := c.userDo(ctx, http.MethodGet, "/v2/me/folders"+folderQuery(cursor, nil), accessToken, nil, &page)
		return page, e
	})
	return foldersView(rows), err
}

// MyFoldersHolding answers "which of my folders already hold this work" in one
// request — the question the add-to-folder picker asks on every game page.
func (c *Client) MyFoldersHolding(ctx context.Context, accessToken string, workID int64) ([]Folder, error) {
	extra := url.Values{"contains_work_id": {strconv.FormatInt(workID, 10)}}
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderWire], error) {
		var page List[folderWire]
		_, e := c.userDo(ctx, http.MethodGet, "/v2/me/folders"+folderQuery(cursor, extra), accessToken, nil, &page)
		return page, e
	})
	return foldersView(rows), err
}

func (c *Client) MyFolderItems(ctx context.Context, accessToken string, folderID int64) ([]FolderItem, error) {
	path := "/v2/me/folders/" + strconv.FormatInt(folderID, 10) + "/items"
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderItemWire], error) {
		var page List[folderItemWire]
		_, e := c.userDo(ctx, http.MethodGet, path+folderQuery(cursor, nil), accessToken, nil, &page)
		return page, e
	})
	return itemsView(rows), err
}

// FolderPreviewItems is one page of a folder's items and nothing more — what a
// shelf card needs to draw a few covers. It takes the reader's token for their
// own folders and the application key for a public one, the same split as the
// full walk.
//
// These are not the newest additions: the catalog orders items by its
// updated_at sync watermark, and the reading order this site shows is applied
// after a full walk. A cover mosaic does not care which four it gets; walking
// every page of every folder to find out would cost a request per hundred
// items per folder on a page that draws one card each.
func (c *Client) FolderPreviewItems(ctx context.Context, accessToken string, folderID int64, limit int) ([]FolderItem, error) {
	q := url.Values{"limit": {strconv.Itoa(min(max(limit, 1), folderPageMax))}}
	id := strconv.FormatInt(folderID, 10)
	var page List[folderItemWire]
	if accessToken != "" {
		if _, err := c.userDo(ctx, http.MethodGet,
			"/v2/me/folders/"+id+"/items?"+q.Encode(), accessToken, nil, &page); err != nil {
			return nil, err
		}
		return itemsView(page.Items), nil
	}
	if err := c.get(ctx, "/v2/folders/"+id+"/items?"+q.Encode(), &page); err != nil {
		return nil, err
	}
	return itemsView(page.Items), nil
}

func (c *Client) MyFolder(ctx context.Context, accessToken string, folderID int64) (*Folder, error) {
	var out folderWire
	if _, err := c.userDo(ctx, http.MethodGet,
		"/v2/me/folders/"+strconv.FormatInt(folderID, 10), accessToken, nil, &out); err != nil {
		return nil, err
	}
	f := out.view()
	return &f, nil
}

// The public lane takes the application key, not the reader's token: a public
// folder is public to signed-out readers too, and folder:read is consent to
// read the bearer's OWN folders, which says nothing about anybody else's.
func (c *Client) PublicFolders(ctx context.Context, ownerUID int64) ([]Folder, error) {
	extra := url.Values{"owner_uid": {strconv.FormatInt(ownerUID, 10)}}
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderWire], error) {
		var page List[folderWire]
		e := c.get(ctx, "/v2/folders"+folderQuery(cursor, extra), &page)
		return page, e
	})
	return foldersView(rows), err
}

func (c *Client) PublicFolder(ctx context.Context, folderID int64) (*Folder, error) {
	var out folderWire
	if err := c.get(ctx, "/v2/folders/"+strconv.FormatInt(folderID, 10), &out); err != nil {
		return nil, err
	}
	f := out.view()
	return &f, nil
}

func (c *Client) PublicFolderItems(ctx context.Context, folderID int64) ([]FolderItem, error) {
	path := "/v2/folders/" + strconv.FormatInt(folderID, 10) + "/items"
	rows, err := walkFolderPages(ctx, func(cursor string) (List[folderItemWire], error) {
		var page List[folderItemWire]
		e := c.get(ctx, path+folderQuery(cursor, nil), &page)
		return page, e
	})
	return itemsView(rows), err
}

func (c *Client) CreateFolder(ctx context.Context, accessToken string, in FolderWrite) (*Folder, error) {
	var out folderWire
	if _, err := c.userDo(ctx, http.MethodPost, "/v2/me/folders", accessToken, in, &out); err != nil {
		return nil, err
	}
	f := out.view()
	return &f, nil
}

func (c *Client) PatchFolder(ctx context.Context, accessToken string, folderID int64, in FolderWrite) (*Folder, error) {
	var out folderWire
	if _, err := c.userDo(ctx, http.MethodPatch,
		"/v2/me/folders/"+strconv.FormatInt(folderID, 10), accessToken, in, &out); err != nil {
		return nil, err
	}
	f := out.view()
	return &f, nil
}

func (c *Client) DeleteFolder(ctx context.Context, accessToken string, folderID int64) error {
	_, err := c.userDo(ctx, http.MethodDelete,
		"/v2/me/folders/"+strconv.FormatInt(folderID, 10), accessToken, nil, nil)
	return err
}

func (c *Client) PutFolderItem(ctx context.Context, accessToken string, folderID, workID int64) error {
	_, err := c.userDo(ctx, http.MethodPut,
		"/v2/me/folders/"+strconv.FormatInt(folderID, 10)+"/items/"+strconv.FormatInt(workID, 10),
		accessToken, nil, nil)
	return err
}

func (c *Client) DeleteFolderItem(ctx context.Context, accessToken string, folderID, workID int64) error {
	_, err := c.userDo(ctx, http.MethodDelete,
		"/v2/me/folders/"+strconv.FormatInt(folderID, 10)+"/items/"+strconv.FormatInt(workID, 10),
		accessToken, nil, nil)
	return err
}

func parseIDOrZero(raw string) int64 {
	n, _ := ParseID(raw)
	return n
}

func foldersView(rows []folderWire) []Folder {
	out := make([]Folder, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.view())
	}
	return out
}

func itemsView(rows []folderItemWire) []FolderItem {
	out := make([]FolderItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.view())
	}
	return out
}
