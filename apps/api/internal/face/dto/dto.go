// Package dto holds the wire types of the public developer-platform face.
//
// They are deliberately separate from the DTOs the site's own frontend uses.
// Those change whenever a page does; these are a contract a third party builds
// against, gated by oasdiff. One shared struct would make every UI tweak a
// breaking API change.
//
// Shapes follow catalog /v2 rather than this site's habits: every object
// carries `object` so a mixed result can be discriminated without looking at
// the URL it came from, every id is a string, and every timestamp is RFC 3339
// in UTC.
package dto

// List is the collection envelope, field for field the one catalog /v2 answers
// with. Total is absent unless the caller asked for it with include_total, and
// Missing only appears on the ids=/refs= batch lane.
type List[T any] struct {
	Object     string    `json:"object"`
	Items      []T       `json:"items"`
	NextCursor *string   `json:"next_cursor"`
	Total      *int64    `json:"total"`
	Missing    *[]string `json:"missing,omitempty"`
}

func NewList[T any](items []T, next *string) List[T] {
	if items == nil {
		items = []T{}
	}
	return List[T]{Object: "list", Items: items, NextCursor: next}
}

// User is the publisher of a page or a resource, as much of them as a public
// read face has any business carrying. The profile itself belongs to OAuth
// (contract C6) and is never stored here.
type User struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

// Patch is one game page on this site: the thing resources hang off, not the
// game itself.
//
// It carries no title, no cover and no tags on purpose. Catalog is the
// existence layer and owns every one of those; this site stores no copy of
// them. The same nmk_ key that reads this face reads /v2/catalog/works, so
// naming a work here and letting the caller resolve it there is one request
// against the authority instead of a second, staler answer from us.
type Patch struct {
	Object string `json:"object"`
	ID     string `json:"id"`

	// The two anchors a caller arrives by. VndbID is this site's own dedupe
	// key and is always present. CatalogWorkID is kept for callers written
	// against the old shape and now always equals ID: a page id IS the catalog
	// work id since migration 037. It went null on placeholder pages and named
	// a different number on the rest, which is what that migration ended.
	VndbID        string  `json:"vndb_id"`
	CatalogWorkID *string `json:"catalog_work_id"`

	// ContentLimit mirrors catalog's verdict for this work in catalog's own
	// vocabulary, sfw or nsfw. Null means this site has not mirrored it yet;
	// catalog remains the authority either way.
	ContentLimit *string `json:"content_limit"`

	ReleaseDate *string  `json:"release_date"`
	Type        []string `json:"type"`
	Language    []string `json:"language"`
	Platform    []string `json:"platform"`

	ResourceCount int `json:"resource_count"`
	DownloadCount int `json:"download_count"`
	ViewCount     int `json:"view_count"`
	FavoriteCount int `json:"favorite_count"`
	CommentCount  int `json:"comment_count"`

	WebURL string `json:"web_url"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// ResourceUpdatedAt is when a resource was last added or changed, which is
	// what `sort=updated` orders by. UpdatedAt moves for page edits too.
	ResourceUpdatedAt string `json:"resource_updated_at"`

	Publisher *User       `json:"publisher,omitempty"`
	Resources *[]Resource `json:"resources,omitempty"`
}

// Resource is one downloadable item on a page.
//
// It carries no download link, no share code and no password, and that is the
// point rather than an omission: revealing a link on this site is a separate,
// rate-limited, per-resource request whose whole purpose is that a link cannot
// be harvested in bulk. The gateway admits any valid key, so a link here would
// hand every key holder the bulk export that endpoint exists to prevent.
// WebURL is where a reader goes to get one.
type Resource struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	PatchID string `json:"patch_id"`

	Name string `json:"name"`
	// Storage is where the bytes live: `s3` for this site's own object store,
	// `user` for a link the publisher hosts elsewhere.
	Storage string `json:"storage"`
	Size    string `json:"size"`
	// Hash is the BLAKE3 of the file, empty on rows uploaded before it was
	// recorded. It is the only stable identity of the bytes themselves.
	Hash                  string `json:"hash"`
	ModelName             string `json:"model_name"`
	LocalizationGroupName string `json:"localization_group_name"`
	// Note is Markdown source, with this site's /image/<hash> tokens already
	// resolved to absolute URLs. Rendering is the caller's.
	Note string `json:"note"`

	Type     []string `json:"type"`
	Language []string `json:"language"`
	Platform []string `json:"platform"`

	DownloadCount int `json:"download_count"`
	LikeCount     int `json:"like_count"`

	WebURL string `json:"web_url"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	Publisher *User `json:"publisher,omitempty"`
}
