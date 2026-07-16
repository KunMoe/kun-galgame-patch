package service

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/url"
	"strconv"

	"kun-galgame-patch-api/internal/trust"
	"kun-galgame-patch-api/internal/trust/dto"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/trustclient"
)

// TrustService is the BFF façade over the infra Trust & Safety client: the
// generic report forwarder (Phase 1) and the moderator-inbox proxy (Phase 3).
type TrustService struct {
	trust *trustclient.Client
	site  string // moyu's catalog_site, to scope the inbox
}

func NewTrustService(trust *trustclient.Client, site string) *TrustService {
	return &TrustService{trust: trust, site: site}
}

// mapAdminErr turns a trustclient AdminError into a moyu AppError, preserving
// the meaningful statuses (409 claim conflict / illegal transition, 400 bad
// decision, 403 missing permission).
func mapAdminErr(err error) *errors.AppError {
	if stderrors.Is(err, trustclient.ErrNotConfigured) {
		return errors.ErrInternal("审核服务暂未启用")
	}
	var ae *trustclient.AdminError
	if stderrors.As(err, &ae) {
		switch ae.Status {
		case http.StatusConflict:
			return errors.ErrConflict("该条目状态已变化，请刷新后重试")
		case http.StatusBadRequest:
			return errors.ErrBadRequest("处置参数无效")
		case http.StatusForbidden:
			return errors.ErrForbidden()
		case http.StatusUnauthorized:
			return errors.ErrUnauthorized()
		case http.StatusNotFound:
			return errors.ErrNotFound("未找到该审核条目")
		}
	}
	return errors.ErrInternal("审核服务请求失败")
}

// ListReviewItems proxies the inbox, forcing site=moyu so a moyu moderator only
// sees moyu's items.
func (s *TrustService) ListReviewItems(
	ctx context.Context, token string, req *dto.ListReviewItemsRequest,
) (json.RawMessage, *errors.AppError) {
	q := url.Values{}
	if s.site != "" {
		q.Set("site", s.site)
	}
	q.Set("status", strconv.Itoa(req.Status))
	q.Set("source", strconv.Itoa(req.Source))
	q.Set("page", strconv.Itoa(req.Page))
	q.Set("limit", strconv.Itoa(req.Limit))

	data, err := s.trust.ListReviewItems(ctx, token, q)
	if err != nil {
		return nil, mapAdminErr(err)
	}
	return data, nil
}

func (s *TrustService) GetReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, *errors.AppError) {
	data, err := s.trust.GetReviewItem(ctx, token, id)
	if err != nil {
		return nil, mapAdminErr(err)
	}
	return data, nil
}

func (s *TrustService) ClaimReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, *errors.AppError) {
	data, err := s.trust.ClaimReviewItem(ctx, token, id)
	if err != nil {
		return nil, mapAdminErr(err)
	}
	return data, nil
}

func (s *TrustService) DecideReviewItem(ctx context.Context, token string, id int64, body []byte) (json.RawMessage, *errors.AppError) {
	data, err := s.trust.DecideReviewItem(ctx, token, id, body)
	if err != nil {
		return nil, mapAdminErr(err)
	}
	return data, nil
}

// Reasons returns the report-reason catalog for the browser dropdown, resolved
// live from the trust registry (global base + moyu's site extensions). Falls
// back to the seeded global constant when trust is unconfigured or unreachable,
// so the report UI always has options.
func (s *TrustService) Reasons(ctx context.Context) []trust.ReportReason {
	views, err := s.trust.ListReportReasons(ctx)
	if err != nil || len(views) == 0 {
		return trust.GlobalReasons
	}
	out := make([]trust.ReportReason, 0, len(views))
	for _, v := range views {
		out = append(out, trust.ReportReason{Key: v.Key, Label: v.NameCN, Severity: v.Severity})
	}
	return out
}

// SubmitReport forwards a report to the trust service with the session user as
// the reporter. subject_kind / subject_id pass through untouched (trust owns
// validation, dedup, and rate-limiting).
func (s *TrustService) SubmitReport(
	ctx context.Context,
	reporterID int,
	req *dto.SubmitReportRequest,
) (*dto.SubmitReportResponse, *errors.AppError) {
	res, err := s.trust.SubmitReport(ctx, trustclient.ReportRequest{
		SubjectKind: req.SubjectKind,
		SubjectID:   req.SubjectID,
		ReasonKey:   req.ReasonKey,
		ReporterID:  int64(reporterID),
		Note:        req.Note,
		Snapshot:    req.Snapshot,
		SubjectURL:  req.SubjectURL,
	})
	if err != nil {
		switch {
		case stderrors.Is(err, trustclient.ErrValidation):
			return nil, errors.ErrBadRequest("举报信息无效，或该内容暂不支持举报")
		case stderrors.Is(err, trustclient.ErrRateLimited):
			return nil, errors.ErrTooManyRequests("举报过于频繁，请稍后再试")
		case stderrors.Is(err, trustclient.ErrNotConfigured):
			return nil, errors.ErrInternal("举报服务暂未启用")
		default:
			// ErrForbidden/ErrUnauthorized are server-side misconfig (site binding /
			// credentials), not the reporter's fault → 500.
			return nil, errors.ErrInternal("举报提交失败，请稍后再试")
		}
	}
	return &dto.SubmitReportResponse{ReportID: res.ReportID}, nil
}
