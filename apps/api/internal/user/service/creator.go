package service

import (
	"context"
	"encoding/json"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/userclient"
)

// Moyu creator-eligibility thresholds — moyu's OWN policy (change freely here;
// OAuth + wiki are untouched). A user may apply if ANY criterion is met:
// ≥3 published patch resources (moyu's own data) OR ≥5 merged PRs (wiki data).
// See docs/auth/01-creator-role-design.md.
const (
	creatorMinMergedPRs = 5
	creatorMinResources = 3
	creatorSource       = "moyu"
)

// CreatorEligibility is the moyu-side eligibility snapshot (counts vs thresholds).
type CreatorEligibility struct {
	Eligible      bool  `json:"eligible"`
	MergedPRs     int64 `json:"merged_prs"`
	Resources     int64 `json:"resources"`
	NeedMergedPRs int   `json:"need_merged_prs"`
	NeedResources int   `json:"need_resources"`
}

func (s *UserService) creatorEligibility(ctx context.Context, userID int) (*CreatorEligibility, *errors.AppError) {
	stats, err := s.wiki.GetUserStats(ctx, userID)
	if err != nil {
		return nil, errors.ErrInternal("获取贡献统计失败")
	}
	resources := s.repo.CountPublishedPatchResources(userID)
	e := &CreatorEligibility{
		MergedPRs:     stats.PRMerged,
		Resources:     resources,
		NeedMergedPRs: creatorMinMergedPRs,
		NeedResources: creatorMinResources,
	}
	e.Eligible = e.MergedPRs >= creatorMinMergedPRs || e.Resources >= creatorMinResources
	return e, nil
}

// CreatorStatus returns the user's eligibility snapshot + current OAuth
// application (nil if never applied).
func (s *UserService) CreatorStatus(ctx context.Context, userID int, token string) (*CreatorEligibility, *userclient.CreatorApplication, *errors.AppError) {
	e, appErr := s.creatorEligibility(ctx, userID)
	if appErr != nil {
		return nil, nil, appErr
	}
	app, err := s.users.GetMyCreatorApplication(ctx, token)
	if err != nil {
		return nil, nil, errors.ErrInternal("获取申请状态失败")
	}
	return e, app, nil
}

// ApplyCreator enforces moyu's eligibility gate, then files the application on
// the central OAuth queue with evidence. OAuth's own guards (already-creator /
// one-pending / cooldown) surface via the returned message.
func (s *UserService) ApplyCreator(ctx context.Context, userID int, token, message string) (*userclient.CreatorApplication, *errors.AppError) {
	e, appErr := s.creatorEligibility(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if !e.Eligible {
		return nil, errors.ErrBadRequest("尚不满足创作者申请条件")
	}
	evidence, _ := json.Marshal(map[string]any{"merged_prs": e.MergedPRs, "resources": e.Resources})
	app, err := s.users.CreateCreatorApplication(ctx, token, creatorSource, evidence, message)
	if err != nil {
		if ce, ok := err.(*userclient.CreatorAPIError); ok {
			return nil, errors.ErrBadRequest(ce.Message)
		}
		return nil, errors.ErrInternal("提交申请失败")
	}
	return app, nil
}
