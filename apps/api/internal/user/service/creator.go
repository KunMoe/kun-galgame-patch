package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/upstream"
	"kun-galgame-patch-api/pkg/userclient"
)

const (
	creatorMinMergedPRs   = 5
	creatorMinResources   = 3
	creatorMinMoemoepoint = 2000
	creatorSource         = "moyu"
)

type CreatorEligibility struct {
	Eligible        bool  `json:"eligible"`
	MergedPRs       int64 `json:"merged_prs"`
	Resources       int64 `json:"resources"`
	Moemoepoint     int64 `json:"moemoepoint"`
	NeedMergedPRs   int   `json:"need_merged_prs"`
	NeedResources   int   `json:"need_resources"`
	NeedMoemoepoint int   `json:"need_moemoepoint"`
}

func (s *UserService) mergedProposalTotal(ctx context.Context, userID int) int64 {
	if s.galgame == nil {
		return 0
	}
	v2 := s.galgame.V2()
	if v2 == nil || !v2.Configured() {
		return 0
	}
	n, err := v2.MergedProposalTotal(ctx, userID, catalogv2.SiteKungal)
	if err != nil {
		slog.Warn("读取合并提案数失败，按 0 计", "user_id", userID, "error", err)
		return 0
	}
	return n
}

func (s *UserService) moemoepointBalance(ctx context.Context, userID int) int {
	balance, err := s.mp.Balance(ctx, userID)
	if err == nil {
		return balance
	}
	userclient.LogFailure(ctx, "moemoepoint balance unavailable; creator eligibility reads the cached one",
		err, "user_id", userID)
	s.db.WithContext(ctx).Table("user").Select("moemoepoint").Where("id = ?", userID).Scan(&balance)
	return balance
}

func (s *UserService) creatorEligibility(ctx context.Context, userID int) (*CreatorEligibility, *errors.AppError) {
	mergedProposals := s.mergedProposalTotal(ctx, userID)
	resources := s.repo.CountPublishedPatchResources(userID)
	e := &CreatorEligibility{
		MergedPRs:       mergedProposals,
		Resources:       resources,
		Moemoepoint:     int64(s.moemoepointBalance(ctx, userID)),
		NeedMergedPRs:   creatorMinMergedPRs,
		NeedResources:   creatorMinResources,
		NeedMoemoepoint: creatorMinMoemoepoint,
	}
	e.Eligible = e.MergedPRs >= creatorMinMergedPRs ||
		e.Resources >= creatorMinResources ||
		e.Moemoepoint >= creatorMinMoemoepoint
	return e, nil
}

func (s *UserService) CreatorStatus(ctx context.Context, userID int, token string) (*CreatorEligibility, *userclient.CreatorApplication, *errors.AppError) {
	e, appErr := s.creatorEligibility(ctx, userID)
	if appErr != nil {
		return nil, nil, appErr
	}
	app, err := s.users.GetMyCreatorApplication(ctx, token)
	if err != nil {
		return nil, nil, creatorFailure(ctx, userID, err, "获取申请状态失败")
	}
	return e, app, nil
}

func (s *UserService) ApplyCreator(ctx context.Context, userID int, token, message string) (*userclient.CreatorApplication, *errors.AppError) {
	if utf8.RuneCountInString(message) > userclient.CreatorMessageMaxRunes {
		return nil, errors.ErrBadRequest("附言最多 1000 字")
	}
	e, appErr := s.creatorEligibility(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if !e.Eligible {
		return nil, errors.ErrBadRequest("尚不满足创作者申请条件")
	}
	evidence, _ := json.Marshal(map[string]any{"merged_prs": e.MergedPRs, "resources": e.Resources, "moemoepoint": e.Moemoepoint})
	app, err := s.users.CreateCreatorApplication(ctx, token, creatorSource, evidence, message)
	if err != nil {
		return nil, creatorFailure(ctx, userID, err, "提交申请失败")
	}
	return app, nil
}

// creatorFailure passes on only the refusals the reader can act on. The rest
// (a 401 on the token moyu holds, a scope or validation error on moyu's own
// request) used to reach the reader as a 400 carrying OAuth's message.
func creatorFailure(ctx context.Context, userID int, err error, msg string) *errors.AppError {
	switch userclient.CreatorRefusal(err) {
	case userclient.CreatorAlreadyHas:
		return errors.ErrBadRequest("你已经是创作者了")
	case userclient.CreatorAppPending:
		return errors.ErrBadRequest("已有一份待审核的创作者申请")
	case userclient.CreatorAppCooldown:
		return errors.ErrBadRequest("申请被拒绝后需等待冷却期才能重新申请")
	case userclient.AccountBanned:
		return errors.ErrAccountBanned("")
	}
	userclient.LogFailure(ctx, "creator application call failed", err, "user_id", userID)
	switch upstream.KindOf(err) {
	case upstream.Unavailable:
		return errors.New(50300, "登录服务暂不可用，请稍后再试", http.StatusServiceUnavailable)
	case upstream.RateLimited:
		return errors.ErrTooManyRequests("操作过于频繁，请稍后再试")
	}
	return errors.ErrInternal(msg)
}
