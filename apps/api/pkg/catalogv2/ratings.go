package catalogv2

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// RatingChangesLimit is the feed's ceiling; only a full page carries a
// next_cursor.
const RatingChangesLimit = 100

type RatingAspects struct {
	Art         *int `json:"art"`
	Story       *int `json:"story"`
	Music       *int `json:"music"`
	Character   *int `json:"character"`
	Route       *int `json:"route"`
	System      *int `json:"system"`
	Voice       *int `json:"voice"`
	ReplayValue *int `json:"replay_value"`
}

// UserRating is one person's rating of one work, the same on every NextMoe
// site. State is the author's play state on the work as it stands now, empty
// when they have none.
type UserRating struct {
	ID           int64
	WorkID       int64
	UserID       int
	Overall      int
	Aspects      RatingAspects
	Recommend    string
	SpoilerLevel string
	ShortSummary string
	ClientID     string
	Site         string
	State        string
	Completion   *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type workStateWire struct {
	State      string  `json:"state"`
	Completion *string `json:"completion"`
}

type userRatingWire struct {
	ID           string         `json:"id"`
	WorkID       string         `json:"work_id"`
	UserID       string         `json:"user_id"`
	Overall      int            `json:"overall"`
	AspectScores RatingAspects  `json:"aspect_scores"`
	Recommend    string         `json:"recommend"`
	SpoilerLevel string         `json:"spoiler_level"`
	ShortSummary string         `json:"short_summary"`
	ClientID     string         `json:"client_id"`
	Site         string         `json:"site"`
	WorkState    *workStateWire `json:"work_state"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (w *userRatingWire) rating() *UserRating {
	if w == nil {
		return nil
	}
	id, _ := ParseID(w.ID)
	workID, _ := ParseID(w.WorkID)
	userID, _ := ParseID(w.UserID)
	out := &UserRating{
		ID: id, WorkID: workID, UserID: int(userID),
		Overall: w.Overall, Aspects: w.AspectScores, Recommend: w.Recommend,
		SpoilerLevel: w.SpoilerLevel, ShortSummary: w.ShortSummary,
		ClientID: w.ClientID, Site: w.Site,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
	if s := w.WorkState; s != nil {
		out.State, out.Completion = s.State, s.Completion
	}
	return out
}

// RatingInput is a whole rating: it replaces every field, so an aspect left nil
// is unrated afterwards.
type RatingInput struct {
	Overall      int            `json:"overall"`
	AspectScores *RatingAspects `json:"aspect_scores,omitempty"`
	Recommend    string         `json:"recommend"`
	SpoilerLevel string         `json:"spoiler_level,omitempty"`
	ShortSummary string         `json:"short_summary,omitempty"`
}

type WorkStateInput struct {
	State      string `json:"state"`
	Completion string `json:"completion,omitempty"`
}

type PlaytimeInput struct {
	Minutes int `json:"minutes"`
}

// WorkRecordInput replaces the caller's whole record on a work: a nil part is
// deleted upstream. Playtime is this application's own report only.
type WorkRecordInput struct {
	WorkState *WorkStateInput `json:"work_state,omitempty"`
	Playtime  *PlaytimeInput  `json:"playtime,omitempty"`
	Rating    *RatingInput    `json:"rating,omitempty"`
}

// WorkRecord is what the caller has recorded on a work. Minutes is the most any
// application reported, not this site's own report.
type WorkRecord struct {
	State      string
	Completion *string
	Minutes    int
	Rating     *UserRating
}

type workRecordWire struct {
	WorkState *workStateWire `json:"work_state"`
	Playtime  *struct {
		Minutes int `json:"minutes"`
	} `json:"playtime"`
	Rating *userRatingWire `json:"rating"`
}

func (w *workRecordWire) record() *WorkRecord {
	out := &WorkRecord{Rating: w.Rating.rating()}
	if s := w.WorkState; s != nil {
		out.State, out.Completion = s.State, s.Completion
	}
	if p := w.Playtime; p != nil {
		out.Minutes = p.Minutes
	}
	return out
}

func workRecordPath(workID int64) string {
	return "/v2/me/work-records/" + FormatID(workID)
}

func (c *Client) WorkRecord(ctx context.Context, accessToken string, workID int64) (*WorkRecord, error) {
	var out workRecordWire
	if _, err := c.userDo(ctx, http.MethodGet, workRecordPath(workID), accessToken, nil, &out); err != nil {
		return nil, err
	}
	return out.record(), nil
}

func (c *Client) PutWorkRecord(ctx context.Context, accessToken string, workID int64, in WorkRecordInput) (*WorkRecord, error) {
	var out workRecordWire
	if _, err := c.userDo(ctx, http.MethodPut, workRecordPath(workID), accessToken, in, &out); err != nil {
		return nil, err
	}
	return out.record(), nil
}

// DeleteMyRating succeeds also when there was no rating.
func (c *Client) DeleteMyRating(ctx context.Context, accessToken string, workID int64) error {
	_, err := c.userDo(ctx, http.MethodDelete, "/v2/me/ratings/"+FormatID(workID), accessToken, nil, nil)
	return err
}

// ModerationDeleteRating removes a rating from every site. Catalog judges the
// token holder's own catalog.rating.moderate, not any role moyu gave them, and
// answers 403 PERMISSION_REQUIRED without it.
func (c *Client) ModerationDeleteRating(ctx context.Context, accessToken string, ratingID int64) error {
	_, err := c.userDo(ctx, http.MethodDelete, "/v2/moderation/ratings/"+FormatID(ratingID), accessToken, nil, nil)
	return err
}

type RatingChange struct {
	ID     int64
	Gone   bool
	Rating *UserRating
}

type RatingChangePage struct {
	Items      []RatingChange
	NextCursor string
}

// RatingChanges answers every site's ratings on every work, not only the works
// a site carries: letmoe kept only its own works' and lost the ratings of a
// work that arrived after them.
func (c *Client) RatingChanges(ctx context.Context, cursor string) (RatingChangePage, error) {
	q := url.Values{"limit": {strconv.Itoa(RatingChangesLimit)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	var page List[struct {
		ID     string          `json:"id"`
		Gone   bool            `json:"gone"`
		Rating *userRatingWire `json:"rating"`
	}]
	if err := c.get(ctx, "/v2/ratings/changes?"+q.Encode(), &page); err != nil {
		return RatingChangePage{}, err
	}
	out := RatingChangePage{Items: make([]RatingChange, 0, len(page.Items)), NextCursor: page.Next()}
	for _, it := range page.Items {
		id, ok := ParseID(it.ID)
		if !ok {
			slog.Warn("ratings changes feed: dropped a row whose id is not a catalog id", "id", it.ID, "cursor", cursor)
			continue
		}
		out.Items = append(out.Items, RatingChange{ID: id, Gone: it.Gone, Rating: it.Rating.rating()})
	}
	return out, nil
}
