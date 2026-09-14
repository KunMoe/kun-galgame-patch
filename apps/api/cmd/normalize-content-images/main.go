// Command normalize-content-images folds every image address left in stored
// content into the domain-free `/image/<hash>[_variant]` token.
//
// Two shapes, one cause -- content held a LOCATION instead of an identity:
//
//   - `sticker.kungal.com/stickers/KUNgal<sid>/<pid>.webp` named a position in
//     a pack on a site this one does not own. Dead everywhere, and the only
//     thing that can still say which picture it meant is legacy_stickers.go.
//   - `https://<cdn>/aa/bb/<hash>[_v].webp` is alive but carries today's CDN
//     domain into every row that quotes it, which is the same failure one
//     rename away.
//
// Rows are backed up before they are rewritten and the run is a dry run unless
// -apply is passed. History and audit tables (patch_resource_revision.changes,
// patch_resource_file_history.old_content, admin_log.content) are deliberately
// left alone: they record what was written at the time.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// The variant the sticker packs are published at, and therefore the one every
// legacy reference resolves to. `GET /api/v1/editor-packs` reports it as
// `variant: "320"`.
const legacyStickerVariant = "320"

var legacyStickerRegex = regexp.MustCompile(
	`https?://sticker\.kungal\.com/stickers/KUNgal([0-9]+)/([0-9]+)\.webp`)

type target struct{ table, col string }

var targets = []target{
	{"patch_comment", "content"},
	{"patch_resource", "note"},
	{"chat_message", "content"},
	{"chat_message_edit_history", "previous_content"},
	{"user_message", "content"},
}

func splitHashes(raw string) []string { return strings.Fields(raw) }

func legacyStickerHash(sid, pid int) string {
	if sid < 1 || sid > len(legacyPackSizes) {
		return ""
	}
	if pid < 1 || pid > legacyPackSizes[sid-1] {
		return ""
	}
	offset := 0
	for i := 0; i < sid-1; i++ {
		offset += legacyPackSizes[i]
	}
	return legacyStickerHashes[offset+pid-1]
}

type report struct {
	rows, changed, stickerHits, urlHits int
	unmapped                            map[string]int
}

func main() {
	_ = godotenv.Load()

	apply := flag.Bool("apply", false, "write the rewrites; without it the run only reports what it would do")
	backupPath := flag.String("backup", "normalize-content-images-backup.jsonl",
		"JSONL of every row's original value, appended before it is rewritten")
	flag.Parse()

	if total := sum(legacyPackSizes[:]); total != len(legacyStickerHashes) {
		slog.Error("贴纸映射表自检失败", "pack_sizes_total", total, "hashes", len(legacyStickerHashes))
		os.Exit(1)
	}

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)
	markdown.RegisterContentImageHost(cfg.ImageService.CDNBase)
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	var backup *os.File
	if *apply {
		f, err := os.OpenFile(*backupPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			slog.Error("打开备份文件失败", "path", *backupPath, "error", err)
			os.Exit(1)
		}
		defer f.Close()
		backup = f
	}

	slog.Info("开始归一化内容图片地址", "apply", *apply, "backup", *backupPath)

	total := report{unmapped: map[string]int{}}
	for _, t := range targets {
		r, err := run(db, t, *apply, backup)
		if err != nil {
			slog.Error("处理失败", "table", t.table, "error", err)
			os.Exit(1)
		}
		slog.Info("处理完成", "table", t.table+"."+t.col,
			"命中行", r.rows, "改写行", r.changed, "贴纸", r.stickerHits, "绝对地址", r.urlHits)
		total.rows += r.rows
		total.changed += r.changed
		total.stickerHits += r.stickerHits
		total.urlHits += r.urlHits
		for k, v := range r.unmapped {
			total.unmapped[k] += v
		}
	}

	if len(total.unmapped) > 0 {
		keys := make([]string, 0, len(total.unmapped))
		for k := range total.unmapped {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		slog.Warn("以下贴纸不在映射表里, 已原样保留", "count", len(keys), "refs", keys)
	}

	verb := "将改写"
	if *apply {
		verb = "已改写"
	}
	fmt.Printf("%s %d 行(命中 %d 行): 贴纸 %d 处, 绝对图床地址 %d 处, 无法映射 %d 种。\n",
		verb, total.changed, total.rows, total.stickerHits, total.urlHits, len(total.unmapped))
}

func run(db *gorm.DB, t target, apply bool, backup *os.File) (report, error) {
	r := report{unmapped: map[string]int{}}

	type row struct {
		ID   int64
		Body string
	}
	var rows []row
	// Both shapes in one pass so a row carrying each is written once.
	if err := db.Table(t.table).
		Select("id, "+t.col+" AS body").
		Where(t.col+" ~ ?", `sticker\.kungal\.com/stickers/|https?://[A-Za-z0-9.-]+/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{64}(_[a-z0-9]+)?\.webp`).
		Order("id ASC").Scan(&rows).Error; err != nil {
		return r, err
	}
	r.rows = len(rows)

	for _, src := range rows {
		body, stickers := rewriteLegacyStickers(src.Body, r.unmapped)
		before := body
		body = markdown.NormalizeContentImageURLs(body)
		urls := countTokenGain(before, body)

		if body == src.Body {
			continue
		}
		r.changed++
		r.stickerHits += stickers
		r.urlHits += urls
		if !apply {
			continue
		}
		if err := writeBackup(backup, t.table, t.col, src.ID, src.Body); err != nil {
			return r, fmt.Errorf("备份 %s#%d: %w", t.table, src.ID, err)
		}
		if err := db.Exec(
			"UPDATE "+t.table+" SET "+t.col+" = ? WHERE id = ?", body, src.ID,
		).Error; err != nil {
			return r, fmt.Errorf("更新 %s#%d: %w", t.table, src.ID, err)
		}
	}
	return r, nil
}

func rewriteLegacyStickers(src string, unmapped map[string]int) (string, int) {
	hits := 0
	out := legacyStickerRegex.ReplaceAllStringFunc(src, func(u string) string {
		m := legacyStickerRegex.FindStringSubmatch(u)
		sid, _ := strconv.Atoi(m[1])
		pid, _ := strconv.Atoi(m[2])
		hash := legacyStickerHash(sid, pid)
		if hash == "" {
			unmapped[fmt.Sprintf("KUNgal%d/%d", sid, pid)]++
			return u
		}
		hits++
		return "/image/" + hash + "_" + legacyStickerVariant
	})
	return out, hits
}

// The normalizer replaces in place, so the number of absolute URLs it folded is
// the number of tokens that appeared.
func countTokenGain(before, after string) int {
	n := strings.Count(after, "/image/") - strings.Count(before, "/image/")
	if n < 0 {
		return 0
	}
	return n
}

func writeBackup(f *os.File, table, col string, id int64, body string) error {
	rec, err := json.Marshal(struct {
		Table  string `json:"table"`
		Column string `json:"column"`
		ID     int64  `json:"id"`
		Body   string `json:"body"`
	}{table, col, id, body})
	if err != nil {
		return err
	}
	_, err = f.Write(append(rec, '\n'))
	return err
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}
