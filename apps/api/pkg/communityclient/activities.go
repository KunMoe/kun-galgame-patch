package communityclient

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

const (
	ActivityCreated  = "created"
	ActivityUpdated  = "updated"
	ActivityRemoved  = "removed"
	ActivityRestored = "restored"
	ActivityStale    = "stale"
	ActivityInvalid  = "invalid"
)

// ActivityItem is one write to POST /activities. A tombstone (Removed) needs
// only Key, ActorID and Revision.
type ActivityItem struct {
	Key            string     `json:"key"`
	ActorID        int64      `json:"actor_id"`
	Revision       int64      `json:"revision"`
	Removed        bool       `json:"removed,omitempty"`
	Verb           string     `json:"verb,omitempty"`
	ObjectKind     string     `json:"object_kind,omitempty"`
	ObjectLabel    string     `json:"object_label,omitempty"`
	Title          string     `json:"title,omitempty"`
	Excerpt        string     `json:"excerpt,omitempty"`
	URL            string     `json:"url,omitempty"`
	CoverImageHash string     `json:"cover_image_hash,omitempty"`
	WorkID         *int64     `json:"work_id,omitempty"`
	ContentLimit   string     `json:"content_limit,omitempty"`
	Notify         bool       `json:"notify,omitempty"`
	OccurredAt     *time.Time `json:"occurred_at,omitempty"`
}

type activityWriteRequest struct {
	Items []ActivityItem `json:"items"`
}

type ActivityOutcome struct {
	Key     string `json:"key"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}

type ActivityWriteResponse struct {
	Results []ActivityOutcome `json:"results"`
}

// SiteActivity is what community stores for one of this site's keys, as the
// reconcile read returns it.
type SiteActivity struct {
	ID             int64      `json:"id"`
	Key            string     `json:"key"`
	ActorID        int64      `json:"actor_id"`
	Verb           string     `json:"verb"`
	ObjectKind     string     `json:"object_kind"`
	ObjectLabel    string     `json:"object_label"`
	Title          string     `json:"title"`
	Excerpt        string     `json:"excerpt"`
	URL            string     `json:"url"`
	CoverImageHash *string    `json:"cover_image_hash"`
	WorkID         *int64     `json:"work_id"`
	ContentLimit   string     `json:"content_limit"`
	Notify         bool       `json:"notify"`
	OccurredAt     time.Time  `json:"occurred_at"`
	Revision       int64      `json:"revision"`
	Removed        bool       `json:"removed"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	RemovedAt      *time.Time `json:"removed_at"`
}

type SiteActivityPage struct {
	Activities []SiteActivity `json:"activities"`
	NextCursor string         `json:"next_cursor"`
}

type ActivityItemView struct {
	ID             int64     `json:"id"`
	Site           string    `json:"site"`
	Key            string    `json:"key"`
	ActorID        int64     `json:"actor_id"`
	Verb           string    `json:"verb"`
	ObjectKind     string    `json:"object_kind"`
	ObjectLabel    string    `json:"object_label"`
	Title          string    `json:"title"`
	Excerpt        string    `json:"excerpt"`
	URL            string    `json:"url"`
	CoverImageHash *string   `json:"cover_image_hash"`
	WorkID         *int64    `json:"work_id"`
	ContentLimit   string    `json:"content_limit"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// ActivityGroupView is one author's items of one verb and object kind on one
// site on one Beijing day. Items holds at most the newest three.
type ActivityGroupView struct {
	ID          int64              `json:"id"`
	Site        string             `json:"site"`
	ActorID     int64              `json:"actor_id"`
	Verb        string             `json:"verb"`
	ObjectKind  string             `json:"object_kind"`
	ObjectLabel string             `json:"object_label"`
	Day         string             `json:"day"`
	ItemCount   int                `json:"item_count"`
	LatestAt    time.Time          `json:"latest_at"`
	Items       []ActivityItemView `json:"items"`
}

type ActivityGroupPage struct {
	Groups     []ActivityGroupView `json:"groups"`
	NextCursor string              `json:"next_cursor"`
}

type ActivityItemPage struct {
	Items      []ActivityItemView `json:"items"`
	NextCursor string             `json:"next_cursor"`
}

// ActivityUnseen counts to 100; 100 means 100 or more.
type ActivityUnseen struct {
	UnseenCount int        `json:"unseen_count"`
	SeenAt      *time.Time `json:"seen_at"`
}

// ActivitySetting is account-wide: hiding takes the user's activities out of
// everyone else's feed on every site. UpdatedAt is nil until it is first set.
type ActivitySetting struct {
	UserID    int64      `json:"user_id"`
	Hidden    bool       `json:"hidden"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type activitySettingRequest struct {
	Hidden bool `json:"hidden"`
}

type activitySeenRequest struct {
	At *time.Time `json:"at,omitempty"`
}

type ActivitySeen struct {
	SeenAt time.Time `json:"seen_at"`
}

// Feed reads take content_limit "all" or "sfw" — not the sfw/nsfw pair an
// item is written with.
const (
	FeedAll = "all"
	FeedSFW = "sfw"
)

func (c *Client) WriteActivities(ctx context.Context, items []ActivityItem) (*ActivityWriteResponse, error) {
	var out ActivityWriteResponse
	err := c.do(ctx, "writeActivities", http.MethodPost, "/activities", activityWriteRequest{Items: items}, &out)
	return &out, err
}

func (c *Client) ListSiteActivities(ctx context.Context, cursor string, limit int) (*SiteActivityPage, error) {
	var out SiteActivityPage
	q := map[string]string{"cursor": cursor}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	err := c.do(ctx, "listSiteActivities", http.MethodGet, "/activities"+query(q), nil, &out)
	return &out, err
}

func (c *Client) FollowingActivities(ctx context.Context, userID int64, cursor string, limit int, contentLimit string) (*ActivityGroupPage, error) {
	var out ActivityGroupPage
	q := map[string]string{"cursor": cursor, "content_limit": contentLimit}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	err := c.do(ctx, "listFollowingActivities", http.MethodGet,
		"/users/"+itoa(userID)+"/following/activities"+query(q), nil, &out)
	return &out, err
}

// ActivityGroupItems is 404 when the group's author hides their activities,
// unless viewerID is that author.
func (c *Client) ActivityGroupItems(ctx context.Context, groupID, viewerID int64, cursor string, limit int, contentLimit string) (*ActivityItemPage, error) {
	var out ActivityItemPage
	q := map[string]string{"viewer_id": viewer(viewerID), "cursor": cursor, "content_limit": contentLimit}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	err := c.do(ctx, "listActivityGroupItems", http.MethodGet,
		"/activity-groups/"+itoa(groupID)+"/items"+query(q), nil, &out)
	return &out, err
}

func (c *Client) FollowingActivitiesUnseen(ctx context.Context, userID int64, contentLimit string) (*ActivityUnseen, error) {
	var out ActivityUnseen
	q := map[string]string{"content_limit": contentLimit}
	err := c.do(ctx, "getFollowingActivitiesUnseen", http.MethodGet,
		"/users/"+itoa(userID)+"/following/activities/unseen"+query(q), nil, &out)
	return &out, err
}

// MarkFollowingActivitiesSeen moves the account-wide mark; a nil at means now.
func (c *Client) MarkFollowingActivitiesSeen(ctx context.Context, userID int64, at *time.Time) (*ActivitySeen, error) {
	var out ActivitySeen
	err := c.do(ctx, "markFollowingActivitiesSeen", http.MethodPost,
		"/users/"+itoa(userID)+"/following/activities/seen", activitySeenRequest{At: at}, &out)
	return &out, err
}

func (c *Client) ActivitySetting(ctx context.Context, userID int64) (*ActivitySetting, error) {
	var out ActivitySetting
	err := c.do(ctx, "getActivitySetting", http.MethodGet, "/users/"+itoa(userID)+"/activity-settings", nil, &out)
	return &out, err
}

func (c *Client) SetActivityHidden(ctx context.Context, userID int64, hidden bool) (*ActivitySetting, error) {
	var out ActivitySetting
	err := c.do(ctx, "setActivitySetting", http.MethodPut, "/users/"+itoa(userID)+"/activity-settings",
		activitySettingRequest{Hidden: hidden}, &out)
	return &out, err
}
