package deletion

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

const deletionCronName = "oauth_account_deletion"

type dbStore struct {
	db *gorm.DB
}

func (s *dbStore) Cursor(ctx context.Context) (string, error) {
	var rows []string
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COALESCE(last_cursor, '') FROM cron_state WHERE name = ?`, deletionCronName,
	).Scan(&rows).Error; err != nil {
		return "", fmt.Errorf("read deletion cursor: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0], nil
}

func (s *dbStore) SaveCursor(ctx context.Context, cursor string) error {
	return s.db.WithContext(ctx).Exec(`
		INSERT INTO cron_state(name, last_id, last_cursor, updated_at)
		VALUES (?, 0, ?, NOW())
		ON CONFLICT(name) DO UPDATE
		SET last_cursor = EXCLUDED.last_cursor, updated_at = EXCLUDED.updated_at
	`, deletionCronName, cursor).Error
}

func (s *dbStore) Purge(ctx context.Context, userID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var following []int
		if err := tx.Table("user_follow_relation").Where("follower_id = ?", userID).Pluck("following_id", &following).Error; err != nil {
			return err
		}
		var followers []int
		if err := tx.Table("user_follow_relation").Where("following_id = ?", userID).Pluck("follower_id", &followers).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM user_follow_relation WHERE follower_id = ? OR following_id = ?`, userID, userID).Error; err != nil {
			return err
		}
		peers := append(append([]int{}, following...), followers...)
		if len(peers) > 0 {
			if err := tx.Exec(`UPDATE "user" SET
				follower_count  = (SELECT COUNT(*) FROM user_follow_relation WHERE user_follow_relation.following_id = "user".id),
				following_count = (SELECT COUNT(*) FROM user_follow_relation WHERE user_follow_relation.follower_id = "user".id)
				WHERE id IN ?`, peers).Error; err != nil {
				return err
			}
		}

		var liked []int
		if err := tx.Table("user_patch_resource_like_relation").Where("user_id = ?", userID).Pluck("resource_id", &liked).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM user_patch_resource_like_relation WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}
		if len(liked) > 0 {
			if err := tx.Exec(`UPDATE patch_resource SET like_count =
				(SELECT COUNT(*) FROM user_patch_resource_like_relation WHERE user_patch_resource_like_relation.resource_id = patch_resource.id)
				WHERE id IN ?`, liked).Error; err != nil {
				return err
			}
		}

		if err := tx.Exec(`DELETE FROM user_patch_resource_favorite_relation WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM user_patch_favorite_relation WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM user_patch_comment_like_relation WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}

		// Deleting a patch_comment row cascades through parent_id and
		// removes other people's replies.
		if err := tx.Exec(`UPDATE patch_comment SET content = '', edit = '' WHERE user_id = ? AND (content <> '' OR edit <> '')`, userID).Error; err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM chat_message_reaction WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM chat_message_seen WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM chat_message WHERE sender_id = ?`, userID).Error; err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM user_message WHERE recipient_id = ? OR sender_id = ?`, userID, userID).Error; err != nil {
			return err
		}

		// Deleting the user row either fails (patch.user_id RESTRICT) or
		// silently drops every resource they published
		// (patch_resource.user_id CASCADE).
		if err := tx.Exec(`UPDATE "user" SET ip = '', last_login_time = '', follower_count = 0, following_count = 0 WHERE id = ?`, userID).Error; err != nil {
			return err
		}
		return nil
	})
}
