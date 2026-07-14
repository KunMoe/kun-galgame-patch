package dto

// AdminPaginationRequest is the common admin pagination request.
//
// Status is only meaningful for the comment list (the review queue):
// "pending" → status<>0, "approved"/"" → status=0... handled per-endpoint.
// "all" returns both. Other list endpoints ignore it.
type AdminPaginationRequest struct {
	Page   int    `query:"page" validate:"required,min=1"`
	Limit  int    `query:"limit" validate:"required,min=1,max=100"`
	Search string `query:"search" validate:"max=300"`
	Status string `query:"status" validate:"omitempty,oneof=all pending approved"`
}

// AdminUpdateCommentRequest is the request for updating a comment (admin)
type AdminUpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=10007"`
}

// AdminUpdateResourceRequest is the request for updating a resource (admin)
type AdminUpdateResourceRequest struct {
	Note string `json:"note" validate:"max=10007"`
}

// AdminSettingBoolRequest is the request for toggling a boolean admin setting
type AdminSettingBoolRequest struct {
	Enabled bool `json:"enabled"`
}

// AdminStatsRequest is the request for fetching admin stats
type AdminStatsRequest struct {
	Days int `query:"days" validate:"required,min=1"`
}

// AdminStatsResponse is the response for overview stats.
// NOTE: json tags must match the FE keys (app/shared/types/admin.d.ts +
// app/constants/admin.ts ADMIN_STATS_MAP) — `new_resource`, NOT
// `new_patch_resource`, or the "新发布补丁" dashboard card silently renders 0.
type AdminStatsResponse struct {
	NewUser          int64 `json:"new_user"`
	NewActiveUser    int64 `json:"new_active_user"`
	NewGalgame       int64 `json:"new_galgame"`
	NewPatchResource int64 `json:"new_resource"`
	NewComment       int64 `json:"new_comment"`
}

// AdminStatsSumResponse is the response for total stats.
// json tags must match the FE keys (ADMIN_STATS_SUM_MAP + SumData):
// `resource_count` / `comment_count`, NOT `patch_*_count`, or those two
// dashboard cards silently render 0.
type AdminStatsSumResponse struct {
	UserCount          int64 `json:"user_count"`
	GalgameCount       int64 `json:"galgame_count"`
	PatchResourceCount int64 `json:"resource_count"`
	PatchCommentCount  int64 `json:"comment_count"`
}

// PurgeUserRequest is the body for POST /admin/user/:id/purge. When
// PurgeOwnedPatches is true the user's own patches (galgame entries) are
// force-deleted too, cascading every resource (incl. other users') and comment
// beneath them — required to delete the user row when they own any patch
// (patch.user_id is ON DELETE RESTRICT).
type PurgeUserRequest struct {
	PurgeOwnedPatches bool `json:"purge_owned_patches"`
}

// UserPurgePreview is the dry-run breakdown for GET
// /admin/user/:id/purge-preview. Counts reflect what an execute with the same
// purge_owned_patches flag would remove.
type UserPurgePreview struct {
	UserID          int   `json:"user_id"`
	UserExists      bool  `json:"user_exists"`
	Comments        int64 `json:"comments"`
	Resources       int64 `json:"resources"`
	CommentLikes    int64 `json:"comment_likes"`
	ResourceLikes   int64 `json:"resource_likes"`
	Favorites       int64 `json:"favorites"`
	Contributes     int64 `json:"contributes"`
	Following       int64 `json:"following"`
	Followers       int64 `json:"followers"`
	ChatMemberships int64 `json:"chat_memberships"`
	ChatMessages    int64 `json:"chat_messages"`
	PrivateMessages int64 `json:"private_messages"`
	OwnedPatches    int64 `json:"owned_patches"`

	// Additional collateral a force-delete removes (rows under the user's owned
	// patches that may belong to OTHER users). Zero unless previewed with
	// purge_owned_patches=true.
	OwnedPatchResources int64 `json:"owned_patch_resources"`
	OwnedPatchComments  int64 `json:"owned_patch_comments"`

	// MiscTraces: rows in FK-less per-user tables (wiki read-state + file-edit
	// history authored by the user) that the user-row CASCADE can't reach and
	// the purge clears explicitly.
	MiscTraces int64 `json:"misc_traces"`

	// CanDeleteUserRow is false when the user still owns patches and the preview
	// flag is false — DELETE FROM "user" would hit the patch.user_id RESTRICT FK.
	CanDeleteUserRow bool `json:"can_delete_user_row"`
}

// UserPurgeResult summarizes an executed purge.
type UserPurgeResult struct {
	UserID          int  `json:"user_id"`
	UserRowDeleted  bool `json:"user_row_deleted"`
	SessionsRevoked int  `json:"sessions_revoked"`
}
