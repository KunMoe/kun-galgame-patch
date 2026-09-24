package catalogv2

import (
	"context"
	"net/url"
	"strconv"
)

type ObjectSchema struct {
	Object           string        `json:"object"`
	TargetObject     string        `json:"target_object"`
	EntityType       string        `json:"entity_type"`
	Fields           []SchemaField `json:"fields"`
	CreationDisabled bool          `json:"creation_disabled"`
}

type SchemaField struct {
	Key           string `json:"key"`
	FieldType     string `json:"field_type"`
	DiffHint      string `json:"diff_hint"`
	Deprecated    bool   `json:"deprecated"`
	MaxSuppressed int    `json:"max_suppressed"`
	MaxElements   int    `json:"max_elements"`
}

type SnapshotRecord struct {
	Object      string         `json:"object"`
	EntityType  string         `json:"entity_type"`
	EntityID    string         `json:"entity_id"`
	FieldValues map[string]any `json:"field_values"`
}

type ProposalRecord struct {
	Object          string  `json:"object"`
	ID              string  `json:"id"`
	State           string  `json:"state"`
	EntityType      string  `json:"entity_type"`
	EntityID        string  `json:"entity_id"`
	Note            string  `json:"note"`
	ProposerUID     string  `json:"proposer_uid"`
	Site            string  `json:"site"`
	BaseRevisionSeq int     `json:"base_revision_seq"`
	DecidedByUID    *string `json:"decided_by_uid"`
	DecidedAt       *string `json:"decided_at"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	// Both are include-gated, and only the DETAIL face takes the token: the
	// me/moderation LIST lanes are built by proposalFrom, which never populates
	// them, and their spec advertises no include at all — asking for one there is
	// a 400. A listed proposal is hydrated one by one or it has no patch.
	Patch          map[string]any `json:"patch"`
	EffectivePatch map[string]any `json:"effective_patch"`
}

// Without site= the tally counts every tenant's proposals filed under the same
// user id, and the id space is shared across them.
func (c *Client) MergedProposalTotal(ctx context.Context, uid int, site string) (int64, error) {
	if uid <= 0 {
		return 0, nil
	}
	q := url.Values{
		"proposer_uid":  {strconv.Itoa(uid)},
		"state":         {"merged"},
		"include_total": {"true"},
		"limit":         {"1"},
	}
	if site != "" {
		q.Set("site", site)
	}
	var out List[ProposalRecord]
	if err := c.get(ctx, "/v2/catalog/proposals?"+q.Encode(), &out); err != nil {
		return 0, err
	}
	if out.Total != nil {
		return *out.Total, nil
	}
	return int64(len(out.Items)), nil
}

func (c *Client) GetSchema(ctx context.Context, object string) (*ObjectSchema, error) {
	var out ObjectSchema
	if err := c.get(ctx, "/v2/catalog/schemas/"+object, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Snapshot(ctx context.Context, accessToken, object string, id int64) (*SnapshotRecord, error) {
	var out SnapshotRecord
	_, err := c.userDo(ctx, "GET", "/v2/moderation/snapshots/"+object+"/"+FormatID(id), accessToken, nil, &out)
	if err != nil {
		return nil, err
	}
	if out.FieldValues == nil {
		out.FieldValues = map[string]any{}
	}
	return &out, nil
}

func (c *Client) CreateProposal(ctx context.Context, accessToken string, actor int, entityType string, entityID int64, patch map[string]any, note string) (*ProposalRecord, error) {
	var out ProposalRecord
	err := c.userPost(ctx, "/v2/me/proposals", accessToken, actor, map[string]any{
		"entity_type": entityType,
		"entity_id":   FormatID(entityID),
		"patch":       patch,
		"note":        note,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListMyProposals(ctx context.Context, accessToken string, entityID int64, limit int) (*List[ProposalRecord], error) {
	if limit <= 0 {
		limit = 20
	}
	path := "/v2/me/proposals?limit=" + strconv.Itoa(limit)
	if entityID > 0 {
		path += "&entity_id=" + FormatID(entityID)
	}
	var out List[ProposalRecord]
	if _, err := c.userDo(ctx, "GET", path, accessToken, nil, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []ProposalRecord{}
	}
	return &out, nil
}

func (c *Client) GetProposal(ctx context.Context, accessToken string, id int64) (*ProposalRecord, string, error) {
	var out ProposalRecord
	etag, err := c.userDo(ctx, "GET", "/v2/me/proposals/"+FormatID(id)+"?include=patch", accessToken, nil, &out)
	if err != nil {
		return nil, "", err
	}
	return &out, etag, nil
}

func (c *Client) WithdrawProposal(ctx context.Context, accessToken string, id int64) (*ProposalRecord, error) {
	if accessToken == "" {
		return nil, ErrNoAccessToken
	}
	_, etag, err := c.GetProposal(ctx, accessToken, id)
	if err != nil {
		return nil, err
	}
	// infra's cond.go treats only a bare * as the wildcard; the quoted one it
	// used to send matched nothing, so this fallback answered 412 every time.
	if etag == "" {
		etag = "*"
	}
	var out ProposalRecord
	if _, err := c.do(ctx, "PATCH", "/v2/me/proposals/"+FormatID(id), accessToken, etag,
		map[string]any{"state": "withdrawn"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
