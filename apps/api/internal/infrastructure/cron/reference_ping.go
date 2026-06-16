package cron

import (
	"context"

	"kun-galgame-patch-api/pkg/imageclient"

	"gorm.io/gorm"
)

// referencePingBatch is image_service's per-request hash cap.
const referencePingBatch = 1000

// RunReferencePing collects every image_service hash this site still references
// and pings image_service so its GC doesn't reclaim them (cold-storage TTL is
// ~60 days). Per image_service 契约 04 §ref-ping, the set is:
//   - entity columns: doc.banner_image_hash
//   - content tokens: /image/<hash> embedded in user markdown
//     (patch_comment.content, patch_resource.note)
//
// Avatars (OAuth-owned) and galgame covers/screenshots (Wiki-owned) are pinged
// by their owning services, not here. Returns the server-reported updated count
// and the number of hashes image_service didn't recognize (a drift signal).
func RunReferencePing(ctx context.Context, db *gorm.DB, img *imageclient.Client) (updated, notFound int, err error) {
	hashes, err := collectReferencedHashes(db)
	if err != nil {
		return 0, 0, err
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
	//
	// patch_comment / patch_resource are the surfaces the editor actually writes
	// tokens to today; chat_message / doc are included defensively — both are
	// goldmark-rendered, so a token there would resolve and must be kept alive.
	// A missed hash → image GC'd after the ~60d TTL, so over-cover, don't under.
	for _, q := range []struct{ table, col string }{
		{"patch_comment", "content"},
		{"patch_resource", "note"},
		{"chat_message", "content"},
		{"doc", "content"},
	} {
		var hs []string
		sql := "SELECT DISTINCT (regexp_matches(" + q.col +
			", '/image/([0-9a-f]{64})', 'g'))[1] AS h FROM " + q.table +
			" WHERE " + q.col + " LIKE '%/image/%'"
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
