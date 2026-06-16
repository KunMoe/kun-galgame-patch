package cron

import (
	"context"
	"fmt"

	"kun-galgame-patch-api/pkg/imageclient"

	"gorm.io/gorm"
)

// referencePingBatch is image_service's per-request hash cap.
const referencePingBatch = 1000

// ContentColumn is a (table, column) pair whose free-text markdown can hold
// domain-agnostic content image tokens (`/image/<hash>`).
type ContentColumn struct{ Table, Col string }

// ContentTokenColumns is the AUTHORITATIVE list of columns whose `/image/<hash>`
// tokens ref-ping keeps alive. It is the single source of truth shared with
// cmd/image-audit, which fails CI if any OTHER column grows such a token (a
// ref-ping gap) — so this list can't silently drift out of coverage again.
//
//   - patch_comment / patch_resource: the surfaces the editor writes tokens to.
//   - chat_message / doc: goldmark-rendered, so a token there would resolve.
//   - user_message / admin_log: notification previews / audit snapshots that
//     embedded the same tokens after the 2026-06 scrub.
//
// A missed hash → image GC'd after the ~60d cold-storage TTL, so over-cover.
var ContentTokenColumns = []ContentColumn{
	{"patch_comment", "content"},
	{"patch_resource", "note"},
	{"chat_message", "content"},
	{"doc", "content"},
	{"user_message", "content"},
	{"admin_log", "content"},
}

// RunReferencePing collects every image_service hash this site still references
// and pings image_service so its GC doesn't reclaim them (cold-storage TTL is
// ~60 days). Per image_service 契约 04 §ref-ping, the set is:
//   - entity columns: doc.banner_image_hash
//   - content tokens: /image/<hash> in ContentTokenColumns
//
// Avatars (OAuth-owned) and galgame covers/screenshots (Wiki-owned) are pinged
// by their owning services, not here.
//
// LOUD FAILURE: if there are hashes to ping but image_service refreshed NONE
// (updated==0, everything not_found), that's the silent-breakage signature from
// the 2026-06 galgame incident (wrong site/creds, or a regression) — it returns
// an error so the caller alerts instead of logging a cheerful "完成 updated=0".
func RunReferencePing(ctx context.Context, db *gorm.DB, img *imageclient.Client) (updated, notFound int, err error) {
	hashes, err := collectReferencedHashes(db)
	if err != nil {
		return 0, 0, err
	}
	if len(hashes) == 0 {
		return 0, 0, nil // nothing referenced yet — genuinely fine
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

// collectReferencedHashes gathers the distinct, well-formed (64-hex) hashes from
// every moyu-owned surface that references image_service.
func collectReferencedHashes(db *gorm.DB) ([]string, error) {
	set := map[string]struct{}{}
	add := func(h string) {
		if len(h) == 64 {
			set[h] = struct{}{}
		}
	}

	// Entity column: doc banners.
	var bannerHashes []string
	if err := db.Table("doc").
		Where("banner_image_hash ~ '^[0-9a-f]{64}$'").
		Pluck("banner_image_hash", &bannerHashes).Error; err != nil {
		return nil, err
	}
	for _, h := range bannerHashes {
		add(h)
	}

	// Content tokens: /image/<hash> embedded in user markdown. regexp_matches
	// with the 'g' flag yields one row per occurrence; DISTINCT de-dups per
	// table (cross-table dedup happens in `set`).
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
