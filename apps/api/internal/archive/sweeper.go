package archive

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/utils"
	"kun-galgame-patch-api/pkg/ziprange"

	"gorm.io/gorm"
)

const (
	perFileTimeout = 30 * time.Second
	sweepBatch     = 100
)

type Sweeper struct {
	db   *gorm.DB
	art  *artifactclient.Client
	http *http.Client
}

func NewSweeper(db *gorm.DB, art *artifactclient.Client) *Sweeper {
	return &Sweeper{db: db, art: art, http: &http.Client{}}
}

func (s *Sweeper) Configured() bool {
	return s != nil && s.db != nil && s.art != nil && s.art.Configured()
}

type Summary struct {
	Ready, Unsupported, Failed, Deleted int
}

var selectUnlistedSQL = `
SELECT r.artifact_uuid
  FROM patch_resource r
  LEFT JOIN artifact_listing al ON al.artifact_uuid = r.artifact_uuid
 WHERE ` + hostedResource + ` AND ` + utils.LiveWorkResource("r") + `
   AND (al.artifact_uuid IS NULL OR (al.state = 'failed' AND al.updated < now() - interval '1 hour'))
 ORDER BY r.id DESC
 LIMIT ?`

const upsertListingSQL = `
INSERT INTO artifact_listing (artifact_uuid, state, entries, file_count, dir_count, total_bytes, error)
VALUES (?, ?, ?::jsonb, ?, ?, ?, ?)
ON CONFLICT (artifact_uuid) DO UPDATE SET
  state = EXCLUDED.state,
  entries = EXCLUDED.entries,
  file_count = EXCLUDED.file_count,
  dir_count = EXCLUDED.dir_count,
  total_bytes = EXCLUDED.total_bytes,
  error = EXCLUDED.error,
  updated = now()`

const deleteOrphanListingSQL = `
DELETE FROM artifact_listing al
 WHERE NOT EXISTS (SELECT 1 FROM patch_resource r WHERE r.artifact_uuid = al.artifact_uuid)`

func (s *Sweeper) Run(ctx context.Context) (Summary, error) {
	var sum Summary
	if !s.Configured() {
		return sum, nil
	}
	var uuids []string
	if err := s.db.WithContext(ctx).Raw(selectUnlistedSQL, sweepBatch).Scan(&uuids).Error; err != nil {
		return sum, err
	}
	for _, uuid := range uuids {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		res := s.readOne(ctx, uuid)
		if err := s.upsert(ctx, uuid, res); err != nil {
			if ctx.Err() != nil {
				return sum, ctx.Err()
			}
			sum.Failed++
			continue
		}
		switch res.state {
		case "ready":
			sum.Ready++
		case "unsupported":
			sum.Unsupported++
		default:
			sum.Failed++
		}
	}
	del := s.db.WithContext(ctx).Exec(deleteOrphanListingSQL)
	if del.Error != nil {
		return sum, del.Error
	}
	sum.Deleted = int(del.RowsAffected)
	return sum, nil
}

type readResult struct {
	state   string
	listing ziprange.Listing
	errText string
}

func (s *Sweeper) readOne(parent context.Context, uuid string) readResult {
	ctx, cancel := context.WithTimeout(parent, perFileTimeout)
	defer cancel()
	dl, err := s.art.Download(ctx, uuid)
	if err != nil {
		return readResult{state: "failed", errText: err.Error()}
	}
	listing, err := ziprange.List(ctx, s.http, dl.Url)
	if errors.Is(err, ziprange.ErrUnsupported) {
		return readResult{state: "unsupported", errText: err.Error()}
	}
	if err != nil {
		return readResult{state: "failed", errText: err.Error()}
	}
	return readResult{state: "ready", listing: listing}
}

func (s *Sweeper) upsert(ctx context.Context, uuid string, res readResult) error {
	entries := res.listing.Entries
	if entries == nil {
		entries = []ziprange.Entry{}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	l := res.listing
	return s.db.WithContext(ctx).Exec(upsertListingSQL,
		uuid, res.state, string(raw), l.FileCount, l.DirCount, l.TotalBytes, res.errText,
	).Error
}
