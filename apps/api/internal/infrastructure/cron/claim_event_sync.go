package cron

import (
	"context"
	"fmt"
	"log/slog"

	authModel "kun-galgame-patch-api/internal/auth/model"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	userModel "kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// Catalog and retired wiki feeds have unrelated ID spaces; reusing
	// wiki_msg_sync skips or replays events silently.
	claimSyncCronName = "catalog_claim_sync"
	claimSyncSchedule = "*/10 * * * *"
	claimSyncBatch    = 100
	claimSyncMaxPages = 50
	// Every event is applied to the page catalog's work_id names, which is this
	// site's page id since migration 037. product_work_id is the FORUM's page
	// number -- both downstreams answer to catalog site `kungal` and only
	// kungal still carries the legacy gid -- so keying on it would file the
	// event on whatever unrelated game holds that number here. Read without
	// site= the feed answers every tenant as well, which is why the request
	// pins it and applyClaimEvent refuses anything from another one.
	claimSyncSite = catalogv2.SiteKungal
)

func RunClaimEventSync(
	ctx context.Context,
	db *gorm.DB,
	catalog *catalogv2.Client,
	galgame *galgameClient.Client,
) (int, int64, error) {
	if db == nil || catalog == nil || !catalog.Configured() {
		return 0, 0, fmt.Errorf("claim event sync: missing db or catalog client")
	}

	cursor, seeded, err := readClaimCursor(db)
	if err != nil {
		return 0, 0, err
	}
	if !seeded {
		head, herr := catalog.ClaimEventHead(ctx, claimSyncSite)
		if herr != nil {
			return 0, 0, fmt.Errorf("seek claim feed head: %w", herr)
		}
		if werr := writeClaimCursor(db, head); werr != nil {
			return 0, 0, werr
		}
		slog.Info("catalog claim 同步已初始化游标 (不回填历史)", "head", head)
		return 0, head, nil
	}

	applied := 0
	for page := 1; page <= claimSyncMaxPages; page++ {
		feed, ferr := catalog.ClaimEvents(ctx, cursor, claimSyncBatch, claimSyncSite)
		if ferr != nil {
			return applied, cursor, fmt.Errorf("fetch claim feed: %w", ferr)
		}
		if len(feed) == 0 {
			break
		}
		for i := range feed {
			ev := &feed[i]
			next := ev.ID
			txErr := db.Transaction(func(tx *gorm.DB) error {
				if err := applyClaimEvent(ctx, tx, galgame, ev); err != nil {
					return err
				}
				return writeClaimCursorTx(tx, next)
			})
			if txErr != nil {
				return applied, cursor, txErr
			}
			cursor = next
			applied++
		}
		if len(feed) < claimSyncBatch {
			break
		}
	}
	return applied, cursor, nil
}

type claimEffect int

const (
	claimEffectNone claimEffect = iota
	claimEffectApproved
	claimEffectDeclined
	claimEffectBanned
	claimEffectUnbanned
	claimEffectRememberSubmitter
	claimEffectUnknownState
)

func effectOf(ev *catalogv2.ClaimEvent) claimEffect {
	if ev.ProductWorkID == nil || *ev.ProductWorkID <= 0 {
		return claimEffectNone
	}
	switch ev.ToState {
	case catalogv2.ClaimStateLive:
		switch {
		case ev.FromState == nil:
			// A claim born directly into live is an import, not an approval.
			return claimEffectNone
		case *ev.FromState == catalogv2.ClaimStatePending:
			return claimEffectApproved
		case *ev.FromState == catalogv2.ClaimStateHidden:
			return claimEffectUnbanned
		default:
			return claimEffectNone
		}
	case catalogv2.ClaimStateDeclined:
		return claimEffectDeclined
	case catalogv2.ClaimStateHidden:
		return claimEffectBanned
	case catalogv2.ClaimStatePending:
		return claimEffectRememberSubmitter
	case catalogv2.ClaimStateDraft, catalogv2.ClaimStateNone:
		return claimEffectNone
	default:
		return claimEffectUnknownState
	}
}

func applyClaimEvent(
	ctx context.Context,
	tx *gorm.DB,
	galgame *galgameClient.Client,
	ev *catalogv2.ClaimEvent,
) error {
	effect := effectOf(ev)
	if ev.Site != claimSyncSite {
		return nil
	}
	if effect == claimEffectUnknownState {
		slog.Warn("收到未识别的 claim 目标状态, 跳过",
			"event", ev.ID, "state", ev.ToState, "work", ev.WorkID)
		return nil
	}
	if effect == claimEffectNone {
		return nil
	}

	res := tx.Exec(`
		INSERT INTO claim_event_processed(event_id, work_id, to_state, actor_uid)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(event_id) DO NOTHING
	`, ev.ID, ev.WorkID, ev.ToState, ev.ActorUID)
	if res.Error != nil {
		return fmt.Errorf("idempotency insert: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil
	}
	if effect == claimEffectRememberSubmitter {
		return nil
	}

	gid := int(ev.WorkID)
	recipient, err := submitterOf(tx, ev.WorkID)
	if err != nil {
		return fmt.Errorf("look up submitter (work=%d): %w", ev.WorkID, err)
	}
	if recipient <= 0 {
		slog.Error("claim 事件无法归属投稿人, 跳过通知与发奖",
			"event", ev.ID, "work", ev.WorkID, "gid", gid, "to_state", ev.ToState)
		return nil
	}

	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&authModel.User{ID: recipient}).Error; err != nil {
		return fmt.Errorf("ensure recipient user anchor (uid=%d): %w", recipient, err)
	}

	name := claimWorkName(ctx, galgame, gid)
	reason := ""
	if ev.Reason != nil {
		reason = *ev.Reason
	}

	switch effect {
	case claimEffectApproved:
		// The forum pays for the approval. Both sites read this same site=kungal
		// feed and an event does not say which site the submission came from, so
		// paying here too paid all 27 approvals from 2026-08-04 to 2026-09-18
		// twice. The forum sends no notice, so this one stays.
		return writeClaimNotification(tx, recipient, gid,
			fmt.Sprintf("您提交的《%s》已通过审核", name))
	case claimEffectDeclined:
		text := fmt.Sprintf("您提交的《%s》未通过审核", name)
		if reason != "" {
			text += "：" + reason
		}
		return writeClaimNotification(tx, recipient, gid, text)
	case claimEffectBanned:
		if err := tx.Model(&patchModel.Patch{}).Where("id = ?", gid).
			Update("published", false).Error; err != nil {
			return fmt.Errorf("unpublish banned patch: %w", err)
		}
		text := fmt.Sprintf("您的作品《%s》已被封禁", name)
		if reason != "" {
			text += "：" + reason
		}
		return writeClaimNotification(tx, recipient, gid, text)
	case claimEffectUnbanned:
		return writeClaimNotification(tx, recipient, gid,
			fmt.Sprintf("您的作品《%s》已解除封禁", name))
	}
	return nil
}

func submitterOf(tx *gorm.DB, workID int64) (int, error) {
	var uid int
	err := tx.Raw(`
		SELECT actor_uid FROM claim_event_processed
		WHERE work_id = ? AND to_state = ?
		ORDER BY event_id DESC LIMIT 1
	`, workID, catalogv2.ClaimStatePending).Scan(&uid).Error
	return uid, err
}

func claimWorkName(ctx context.Context, galgame *galgameClient.Client, gid int) string {
	if galgame != nil {
		if briefs, err := galgame.GalgameBatch(ctx, []int{gid}, ""); err == nil {
			for i := range briefs {
				if briefs[i].ID != gid {
					continue
				}
				for _, s := range []string{
					briefs[i].NameZhCn, briefs[i].NameZhTw, briefs[i].NameJaJp, briefs[i].NameEnUs,
				} {
					if s != "" {
						return s
					}
				}
			}
		}
	}
	return fmt.Sprintf("#%d", gid)
}

func writeClaimNotification(tx *gorm.DB, recipient, gid int, text string) error {
	return tx.Create(&userModel.UserMessage{
		Type:        "system",
		Content:     text,
		Status:      0,
		Link:        fmt.Sprintf("/galgame/%d", gid),
		SenderID:    nil,
		RecipientID: &recipient,
	}).Error
}

func readClaimCursor(db *gorm.DB) (cursor int64, seeded bool, err error) {
	var rows []int64
	if err := db.Raw(
		`SELECT last_id FROM cron_state WHERE name = ?`, claimSyncCronName,
	).Scan(&rows).Error; err != nil {
		return 0, false, fmt.Errorf("read claim cron cursor: %w", err)
	}
	if len(rows) == 0 {
		return 0, false, nil
	}
	return rows[0], true, nil
}

func writeClaimCursor(db *gorm.DB, id int64) error { return writeClaimCursorTx(db, id) }

func writeClaimCursorTx(tx *gorm.DB, id int64) error {
	return tx.Exec(`
		INSERT INTO cron_state(name, last_id, updated_at)
		VALUES (?, ?, NOW())
		ON CONFLICT(name) DO UPDATE
		SET last_id = EXCLUDED.last_id, updated_at = EXCLUDED.updated_at
	`, claimSyncCronName, id).Error
}
