package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/face/dto"
	"kun-galgame-patch-api/pkg/problem"

	"github.com/gofiber/fiber/v3"
)

// ChangeCursor is a position in the feed: the last (txid, seq) read.
type ChangeCursor struct {
	Txid int64
	Seq  int64
}

func (p ChangeCursor) Encode() *string {
	out := "cur_" + base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d.%d", p.Txid, p.Seq))
	return &out
}

func ParseChangeCursor(c fiber.Ctx) (ChangeCursor, *problem.Problem) {
	raw := c.Query("cursor")
	if raw == "" {
		return ChangeCursor{}, nil
	}
	invalid := problem.Param(problem.CodeInvalidCursor, "cursor", problem.ReasonInvalidFormat,
		"pass the next_cursor from a previous page of this feed")
	payload, ok := strings.CutPrefix(raw, "cur_")
	if !ok {
		return ChangeCursor{}, invalid
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return ChangeCursor{}, invalid
	}
	txid, seq, ok := strings.Cut(string(decoded), ".")
	if !ok {
		return ChangeCursor{}, invalid
	}
	var pos ChangeCursor
	if pos.Txid, err = strconv.ParseInt(txid, 10, 64); err != nil || pos.Txid < 0 {
		return ChangeCursor{}, invalid
	}
	if pos.Seq, err = strconv.ParseInt(seq, 10, 64); err != nil || pos.Seq < 0 {
		return ChangeCursor{}, invalid
	}
	return pos, nil
}

// ListChanges always answers a next_cursor, the position after the last item
// or the one asked for: a feed is resumed, never finished.
func (s *Service) ListChanges(_ context.Context, after ChangeCursor, limit int) (dto.List[dto.ResourceChange], *problem.Problem) {
	rows, err := s.repo.ListChanges(after.Txid, after.Seq, limit)
	if err != nil {
		return dto.List[dto.ResourceChange]{}, unavailable(err)
	}
	items := make([]dto.ResourceChange, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.ResourceChange{
			Object:        "patch_resource_change",
			Seq:           strconv.FormatInt(r.Seq, 10),
			ResourceID:    strconv.Itoa(r.ResourceID),
			CatalogWorkID: strconv.Itoa(r.PatchID),
			Storage:       r.Storage,
			Kind:          r.Kind,
			ChangedAt:     stamp(r.ChangedAt),
		})
		after = ChangeCursor{Txid: r.Txid, Seq: r.Seq}
	}
	return dto.NewList(items, after.Encode()), nil
}
