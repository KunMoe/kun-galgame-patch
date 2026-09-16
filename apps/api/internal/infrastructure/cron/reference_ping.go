package cron

import (
	"context"
	"fmt"

	"kun-galgame-patch-api/pkg/imageclient"

	"gorm.io/gorm"
)

const referencePingBatch = 1000

type ContentColumn struct{ Table, Col string }

// No patch_comment: comment bodies live in the community primitive since the
// cutover and no SQL here can see them. CommentImages is how they are swept —
// without it every image ever posted in a comment stops being referenced and is
// eventually collected.
var ContentTokenColumns = []ContentColumn{
	{"patch_resource", "note"},
	{"chat_message", "content"},
	{"doc", "content"},
	{"user_message", "content"},
	{"admin_log", "content"},
}

// CommentImages sweeps the comment walls for the images they embed. Nil skips
// the sweep, which is what an unconfigured community client amounts to.
type CommentImages interface {
	CollectImageHashes(ctx context.Context) ([]string, error)
}

func RunReferencePing(ctx context.Context, db *gorm.DB, img *imageclient.Client, comments CommentImages) (updated, notFound int, err error) {
	hashes, err := collectReferencedHashes(db)
	if err != nil {
		return 0, 0, err
	}
	if comments != nil {
		commentHashes, cerr := comments.CollectImageHashes(ctx)
		if cerr != nil {
			// Half a sweep pings half the images, and the unpinged half is what
			// gets collected. Fail the run instead.
			return 0, 0, cerr
		}
		hashes = append(hashes, commentHashes...)
	}
	if len(hashes) == 0 {
		return 0, 0, nil
	}
	for start := 0; start < len(hashes); start += referencePingBatch {
		end := min(start+referencePingBatch, len(hashes))
		res, perr := img.ReferencePing(ctx, hashes[start:end])
		if perr != nil {
			return updated, notFound, perr
		}
		updated += int(res.Updated)
		notFound += len(res.NotFound)
	}
	if updated == 0 {
		return updated, notFound, fmt.Errorf(
			"ref-ping refreshed 0 of %d referenced hashes (all not_found=%d) — image_service may be misconfigured (wrong site/creds) or hashes drifted; NOT a healthy run",
			len(hashes), notFound)
	}
	return updated, notFound, nil
}

func collectReferencedHashes(db *gorm.DB) ([]string, error) {
	set := map[string]struct{}{}
	add := func(h string) {
		if len(h) == 64 {
			set[h] = struct{}{}
		}
	}

	var bannerHashes []string
	if err := db.Table("doc").
		Where("banner_image_hash ~ '^[0-9a-f]{64}$'").
		Pluck("banner_image_hash", &bannerHashes).Error; err != nil {
		return nil, err
	}
	for _, h := range bannerHashes {
		add(h)
	}

	for _, q := range ContentTokenColumns {
		var hs []string
		sql := "SELECT DISTINCT (regexp_matches(" + q.Col +
			", '/image/([0-9a-f]{64})', 'g'))[1] AS h FROM " + q.Table +
			" WHERE " + q.Col + " LIKE '%/image/%'"
		if err := db.Raw(sql).Scan(&hs).Error; err != nil {
			return nil, err
		}
		for _, h := range hs {
			add(h)
		}
	}

	out := make([]string, 0, len(set))
	for h := range set {
		out = append(out, h)
	}
	return out, nil
}
