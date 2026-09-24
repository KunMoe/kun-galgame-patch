package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strconv"

	"kun-galgame-patch-api/internal/trust"
	"kun-galgame-patch-api/internal/trust/dto"
	"kun-galgame-patch-api/pkg/trustclient"
	"kun-galgame-patch-api/pkg/upstream"
)

type TrustService struct {
	trust *trustclient.Client
	site  string
}

func NewTrustService(trust *trustclient.Client, site string) *TrustService {
	return &TrustService{trust: trust, site: site}
}

func (s *TrustService) ListReviewItems(
	ctx context.Context, token string, req *dto.ListReviewItemsRequest,
) (json.RawMessage, error) {
	q := url.Values{}
	if s.site != "" {
		q.Set("site", s.site)
	}
	q.Set("status", strconv.Itoa(req.Status))
	q.Set("source", strconv.Itoa(req.Source))
	q.Set("page", strconv.Itoa(req.Page))
	q.Set("limit", strconv.Itoa(req.Limit))
	return s.trust.ListReviewItems(ctx, token, q)
}

func (s *TrustService) GetReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	return s.trust.GetReviewItem(ctx, token, id)
}

func (s *TrustService) ClaimReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	return s.trust.ClaimReviewItem(ctx, token, id)
}

func (s *TrustService) DecideReviewItem(ctx context.Context, token string, id int64, body []byte) (json.RawMessage, error) {
	return s.trust.DecideReviewItem(ctx, token, id, body)
}

// Reasons falls back to the seeded list only while the trust service is
// unreachable or unconfigured. Any other failure is moyu's credential or
// request and is returned, so it reaches the log instead of hiding behind a
// list that looks healthy.
func (s *TrustService) Reasons(ctx context.Context) ([]trust.ReportReason, error) {
	views, err := s.trust.ListReportReasons(ctx)
	switch {
	case err == nil:
	case errors.Is(err, trustclient.ErrNotConfigured):
		return trust.GlobalReasons, nil
	case upstream.KindOf(err) == upstream.Unavailable:
		slog.Warn("trust report reasons unavailable; serving the seeded list", "error", err)
		return trust.GlobalReasons, nil
	default:
		return nil, err
	}
	if len(views) == 0 {
		return trust.GlobalReasons, nil
	}
	out := make([]trust.ReportReason, 0, len(views))
	for _, v := range views {
		out = append(out, trust.ReportReason{Key: v.Key, Label: v.NameCN, Severity: v.Severity})
	}
	return out, nil
}

func (s *TrustService) SubmitReport(
	ctx context.Context,
	reporterID int,
	req *dto.SubmitReportRequest,
) (*dto.SubmitReportResponse, error) {
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
		return nil, err
	}
	return &dto.SubmitReportResponse{ReportID: res.ReportID}, nil
}
