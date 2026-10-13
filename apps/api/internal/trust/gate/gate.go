package gate

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-patch-api/pkg/trustclient"
)

const (
	SubjectKindGalgameRating = "galgame_rating"

	DecisionAllow = "allow"
	DecisionDeny  = "deny"
	DecisionHold  = "hold"

	checkTimeout = 500 * time.Millisecond
	scanTimeout  = 5 * time.Second
)

type Client interface {
	Configured() bool
	Check(ctx context.Context, req trustclient.CheckRequest) (*trustclient.CheckResult, error)
	Scan(ctx context.Context, req trustclient.ScanRequest) (*trustclient.ScanResult, error)
}

type Gate struct {
	client Client
}

func New(client Client) *Gate {
	return &Gate{client: client}
}

func (g *Gate) enabled() bool {
	return g != nil && g.client != nil && g.client.Configured()
}

func (g *Gate) Denied(ctx context.Context, subjectKind, text string, authorID int) bool {
	if !g.enabled() || text == "" {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	author := int64(authorID)
	res, err := g.client.Check(cctx, trustclient.CheckRequest{Text: text, AuthorID: &author})
	if err != nil {
		slog.Warn("trust check failed open", "subject_kind", subjectKind, "error", err)
		return false
	}
	if res.Decision == DecisionHold {
		slog.Info("trust check hold", "subject_kind", subjectKind, "author_id", authorID, "matched", res.Matched)
	}
	return res.Decision == DecisionDeny
}

func (g *Gate) ScanBg(subjectKind string, subjectID int64, text string, authorID int) {
	if !g.enabled() || text == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
		defer cancel()
		author := int64(authorID)
		id := strconv.FormatInt(subjectID, 10)
		_, err := g.client.Scan(ctx, trustclient.ScanRequest{
			SubjectKind: subjectKind, SubjectID: id, Text: text, AuthorID: &author,
		})
		if err != nil {
			slog.Warn("trust scan failed", "subject_kind", subjectKind, "subject_id", id, "error", err)
		}
	}()
}
