package catalogv2

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"kun-galgame-patch-api/pkg/upstream"
)

const WorkRecordsLimitMax = 100

type WorkRecordsQuery struct {
	States       []string
	ContentLimit string
	Cursor       string
	Limit        int
	Facets       bool
}

type ListedWorkRecord struct {
	WorkID int64
	WorkRecord
}

type WorkRecordPage struct {
	Items       []ListedWorkRecord
	NextCursor  string
	Total       int64
	StateCounts map[string]int
}

func (q WorkRecordsQuery) values() url.Values {
	v := url.Values{}
	if len(q.States) > 0 {
		v.Set("state", strings.Join(q.States, ","))
	}
	if q.ContentLimit != "" {
		v.Set("content_limit", q.ContentLimit)
	}
	if q.Cursor != "" {
		v.Set("cursor", q.Cursor)
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(min(q.Limit, WorkRecordsLimitMax)))
	}
	if q.Facets {
		v.Set("facets", "state")
		v.Set("include_total", "true")
	}
	return v
}

// WorkRecords is someone else's library, which an account that keeps its play
// records private refuses with PLAY_RECORDS_PRIVATE.
func (c *Client) WorkRecords(ctx context.Context, userID int, q WorkRecordsQuery) (WorkRecordPage, error) {
	v := q.values()
	v.Set("user_id", strconv.Itoa(userID))
	return c.workRecords(ctx, "/v2/work-records?"+v.Encode(), "")
}

// MyWorkRecords answers whatever the privacy switch says.
func (c *Client) MyWorkRecords(ctx context.Context, accessToken string, q WorkRecordsQuery) (WorkRecordPage, error) {
	if accessToken == "" {
		return WorkRecordPage{}, ErrNoAccessToken
	}
	return c.workRecords(ctx, "/v2/me/work-records?"+q.values().Encode(), accessToken)
}

func (c *Client) workRecords(ctx context.Context, path, accessToken string) (WorkRecordPage, error) {
	var page List[struct {
		WorkID string `json:"work_id"`
		workRecordWire
	}]
	if _, err := c.do(ctx, http.MethodGet, path, accessToken, "", nil, &page); err != nil {
		return WorkRecordPage{}, err
	}
	out := WorkRecordPage{
		Items:      make([]ListedWorkRecord, 0, len(page.Items)),
		NextCursor: page.Next(),
	}
	if page.Total != nil {
		out.Total = *page.Total
	}
	for i := range page.Items {
		id, ok := ParseID(page.Items[i].WorkID)
		if !ok {
			continue
		}
		out.Items = append(out.Items, ListedWorkRecord{WorkID: id, WorkRecord: *page.Items[i].record()})
	}
	if page.Facets != nil {
		out.StateCounts = map[string]int{}
		for _, b := range (*page.Facets)["state"] {
			out.StateCounts[b.Value] = b.Count
		}
	}
	return out, nil
}

func IsPlayRecordsPrivate(err error) bool {
	e, ok := upstream.As(err)
	return ok && e.Service == service && e.Code == CodePlayRecordsPrivate
}

type privacyWire struct {
	PlayRecords string `json:"play_records"`
}

func (c *Client) PlayRecordsPrivate(ctx context.Context, accessToken string) (bool, error) {
	var out privacyWire
	if _, err := c.userDo(ctx, http.MethodGet, "/v2/me/privacy", accessToken, nil, &out); err != nil {
		return false, err
	}
	return out.PlayRecords == "private", nil
}

func (c *Client) SetPlayRecordsPrivate(ctx context.Context, accessToken string, private bool) (bool, error) {
	in := privacyWire{PlayRecords: "public"}
	if private {
		in.PlayRecords = "private"
	}
	var out privacyWire
	if _, err := c.userDo(ctx, http.MethodPatch, "/v2/me/privacy", accessToken, in, &out); err != nil {
		return false, err
	}
	return out.PlayRecords == "private", nil
}
