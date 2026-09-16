// Package model holds the one local table that survives the move of every
// comment wall to the community primitive. It stores no comment: the post, and
// the reactions on it, live in kun_community.
package model

// PatchCommentCommunityMap is the old patch_comment id -> migrated post. Every
// link minted before the cutover addresses #comment-<old id>, and the import
// tool is what fills this in.
type PatchCommentCommunityMap struct {
	OldCommentID int   `gorm:"primaryKey;column:old_comment_id" json:"old_comment_id"`
	ThreadID     int64 `gorm:"column:thread_id" json:"thread_id"`
	PostID       int64 `gorm:"column:post_id" json:"post_id"`
	GalgameID    int   `gorm:"column:galgame_id" json:"galgame_id"`
	ResourceID   *int  `gorm:"column:resource_id" json:"resource_id"`
}

func (PatchCommentCommunityMap) TableName() string { return "patch_comment_community_map" }
