package dto

type AdminPaginationRequest struct {
	Page   int    `query:"page" validate:"required,min=1"`
	Limit  int    `query:"limit" validate:"required,min=1,max=100"`
	Search string `query:"search" validate:"max=300"`
	Status string `query:"status" validate:"omitempty,oneof=all pending approved"`
}

type AdminUpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=10007"`
}

type AdminUpdateResourceRequest struct {
	Note string `json:"note" validate:"max=10007"`
}

type AdminSettingBoolRequest struct {
	Enabled bool `json:"enabled"`
}

type AdminStatsRequest struct {
	Days int `query:"days" validate:"required,min=1"`
}

type AdminStatsResponse struct {
	NewUser          int64 `json:"new_user"`
	NewActiveUser    int64 `json:"new_active_user"`
	NewGalgame       int64 `json:"new_galgame"`
	NewPatchResource int64 `json:"new_resource"`
	NewComment       int64 `json:"new_comment"`
}

type AdminStatsSumResponse struct {
	UserCount          int64 `json:"user_count"`
	GalgameCount       int64 `json:"galgame_count"`
	PatchResourceCount int64 `json:"resource_count"`
	PatchCommentCount  int64 `json:"comment_count"`
}

type PurgeUserRequest struct {
	PurgeOwnedPatches bool `json:"purge_owned_patches"`
}

type UserPurgePreview struct {
	UserID          int   `json:"user_id"`
	UserExists      bool  `json:"user_exists"`
	Comments        int64 `json:"comments"`
	Resources       int64 `json:"resources"`
	CommentLikes    int64 `json:"comment_likes"`
	ResourceLikes   int64 `json:"resource_likes"`
	Contributes     int64 `json:"contributes"`
	Following       int64 `json:"following"`
	Followers       int64 `json:"followers"`
	ChatMemberships int64 `json:"chat_memberships"`
	ChatMessages    int64 `json:"chat_messages"`
	PrivateMessages int64 `json:"private_messages"`
	OwnedPatches    int64 `json:"owned_patches"`

	OwnedPatchResources int64 `json:"owned_patch_resources"`
	OwnedPatchComments  int64 `json:"owned_patch_comments"`

	MiscTraces int64 `json:"misc_traces"`

	// Read, never deleted — see AdminService.catalogFolders for why a purge
	// preview counts something the purge does not touch.
	CatalogFolders     int64  `json:"catalog_folders"`
	CatalogFolderItems int64  `json:"catalog_folder_items"`
	CatalogFolderError string `json:"catalog_folder_error,omitempty"`

	CanDeleteUserRow bool `json:"can_delete_user_row"`
}

type UserPurgeResult struct {
	UserID          int  `json:"user_id"`
	UserRowDeleted  bool `json:"user_row_deleted"`
	SessionsRevoked int  `json:"sessions_revoked"`
}
