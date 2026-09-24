package catalogv2

import (
	"context"
	"net/http"
)

const (
	ClaimStateNone     = "none"
	ClaimStateLive     = "live"
	ClaimStateDraft    = "draft"
	ClaimStatePending  = "pending"
	ClaimStateDeclined = "declined"
	ClaimStateHidden   = "hidden"
)

// PATCH /v2/me/claims/{id} takes a transition target, not a claim state: draft
// is the older spelling of the same executor action and both still answer.
const ClaimTargetWithdrawn = "withdrawn"

// moyu's OAuth client carries catalog_site=kungal, so the claims it writes and
// the events it reads are recorded under that tenant and their product_work_id
// is a moyu patch id. Wave 161 renamed the only other value the column has ever
// held (galgame_wiki → kungal).
const SiteKungal = "kungal"

const EntityTypeWork = "catalog.work"

type ClaimEventRef struct {
	Object    string  `json:"object"`
	ID        string  `json:"id"`
	FromState *string `json:"from_state"`
	ToState   string  `json:"to_state"`
	Reason    *string `json:"reason"`
	ActorUID  string  `json:"actor_uid"`
	CreatedAt string  `json:"created_at"`
}

type ClaimRecord struct {
	Object        string         `json:"object"`
	ID            string         `json:"id"`
	State         string         `json:"state"`
	DisplayName   string         `json:"display_name"`
	Site          string         `json:"site"`
	ProductWorkID *string        `json:"product_work_id"`
	LastEvent     *ClaimEventRef `json:"last_event"`
	FirstActedAt  *string        `json:"first_acted_at"`
	ActedCount    *int           `json:"acted_count"`
}

func (r ClaimRecord) WorkID() int64 {
	id, _ := ParseID(r.ID)
	return id
}

func (r ClaimRecord) ProductID() *int64 {
	if r.ProductWorkID == nil {
		return nil
	}
	id, ok := ParseID(*r.ProductWorkID)
	if !ok {
		return nil
	}
	return &id
}

func (r ClaimRecord) LastReason() *string {
	if r.LastEvent == nil {
		return nil
	}
	return r.LastEvent.Reason
}

func (c *Client) CreateClaim(ctx context.Context, accessToken, idemKey string, workID, siteWorkID int64) (*ClaimRecord, error) {
	body := map[string]any{"work_id": FormatID(workID)}
	if siteWorkID > 0 {
		body["site_work_id"] = FormatID(siteWorkID)
	}
	var out ClaimRecord
	if err := c.userPost(ctx, "/v2/me/claims", accessToken, idemKey, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// The map's catalog.work.display_name is only a seed: applyTitles runs after
// olang inside the same transaction and rewrites the row to the official title
// in that language, so the wizard's zh-Hans name with olang=ja is born under the
// Japanese title. That was v1's behaviour too; sending display_name at the top
// level instead would pin the seed and change it.
func (c *Client) MintClaim(ctx context.Context, accessToken, idemKey string, fieldValues map[string]any) (*ClaimRecord, error) {
	var out ClaimRecord
	if err := c.userPost(ctx, "/v2/me/claims", accessToken, idemKey,
		map[string]any{"field_values": fieldValues}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ifMatch is required. "*" takes whatever state the claim is in; pass the ETag
// from GetMyClaim when the decision that follows depends on which state that
// was. The answer carries no last_event — the claim read behind PATCH does not
// join the event table — so a caller that needs the prior state must have read
// it beforehand.
func (c *Client) PatchClaim(ctx context.Context, accessToken string, workID int64, state, ifMatch string) (*ClaimRecord, error) {
	if accessToken == "" {
		return nil, ErrNoAccessToken
	}
	var out ClaimRecord
	if _, err := c.do(ctx, http.MethodPatch, "/v2/me/claims/"+FormatID(workID), accessToken, ifMatch,
		map[string]any{"state": state}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Owner-fenced: catalog answers only claims the bearer both acted on and owns,
// so a 404 here is also the authorization refusal.
func (c *Client) GetMyClaim(ctx context.Context, accessToken string, workID int64) (*ClaimRecord, string, error) {
	var out ClaimRecord
	etag, err := c.userDo(ctx, http.MethodGet, "/v2/me/claims/"+FormatID(workID), accessToken, nil, &out)
	if err != nil {
		return nil, "", err
	}
	return &out, etag, nil
}

// Soft-deletes the catalog work behind the claim, not just the claim. Catalog
// allows it only on a draft the bearer owns, and writes no claim event.
func (c *Client) DeleteClaim(ctx context.Context, accessToken string, workID int64) error {
	_, err := c.userDo(ctx, http.MethodDelete, "/v2/me/claims/"+FormatID(workID), accessToken, nil, nil)
	return err
}
