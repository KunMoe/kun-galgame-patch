package communityclient

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// AnchorPresentation names the page a comment wall hangs on. Community knows a
// wall only by its anchor, and titles and links the comments it projects into
// the following feed from this. A tombstone (Removed) needs only the anchor and
// Revision; the outcomes are the Activity* ones.
type AnchorPresentation struct {
	AnchorKind     int32  `json:"anchor_kind"`
	AnchorID       string `json:"anchor_id"`
	Revision       int64  `json:"revision"`
	Removed        bool   `json:"removed,omitempty"`
	Title          string `json:"title,omitempty"`
	URL            string `json:"url,omitempty"`
	CoverImageHash string `json:"cover_image_hash,omitempty"`
	WorkID         *int64 `json:"work_id,omitempty"`
	ContentLimit   string `json:"content_limit,omitempty"`
}

type anchorPresentationWriteRequest struct {
	Items []AnchorPresentation `json:"items"`
}

type AnchorPresentationOutcome struct {
	AnchorKind int32  `json:"anchor_kind"`
	AnchorID   string `json:"anchor_id"`
	Outcome    string `json:"outcome"`
	Reason     string `json:"reason,omitempty"`
}

type AnchorPresentationWriteResponse struct {
	Results []AnchorPresentationOutcome `json:"results"`
}

// StoredAnchorPresentation is what community holds for one of this site's
// anchors. A tombstone's Title, URL, cover and WorkID are cleared.
type StoredAnchorPresentation struct {
	AnchorKind     int32      `json:"anchor_kind"`
	AnchorID       string     `json:"anchor_id"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	CoverImageHash *string    `json:"cover_image_hash"`
	WorkID         *int64     `json:"work_id"`
	ContentLimit   string     `json:"content_limit"`
	Revision       int64      `json:"revision"`
	Removed        bool       `json:"removed"`
	UpdatedAt      time.Time  `json:"updated_at"`
	RemovedAt      *time.Time `json:"removed_at"`
}

type AnchorPresentationPage struct {
	Presentations []StoredAnchorPresentation `json:"presentations"`
	NextCursor    string                     `json:"next_cursor"`
}

func (c *Client) WriteAnchorPresentations(ctx context.Context, items []AnchorPresentation) (*AnchorPresentationWriteResponse, error) {
	var out AnchorPresentationWriteResponse
	err := c.do(ctx, "writeAnchorPresentations", http.MethodPut, "/anchor-presentations",
		anchorPresentationWriteRequest{Items: items}, &out)
	return &out, err
}

func (c *Client) ListAnchorPresentations(ctx context.Context, cursor string, limit int) (*AnchorPresentationPage, error) {
	var out AnchorPresentationPage
	q := map[string]string{"cursor": cursor}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	err := c.do(ctx, "listAnchorPresentations", http.MethodGet, "/anchor-presentations"+query(q), nil, &out)
	return &out, err
}
