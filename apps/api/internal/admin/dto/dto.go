package dto

// AdminPaginationRequest is the common admin pagination request
type AdminPaginationRequest struct {
	Page   int    `query:"page" validate:"required,min=1"`
	Limit  int    `query:"limit" validate:"required,min=1,max=100"`
	Search string `query:"search" validate:"max=300"`
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

// AdminStatsResponse is the response for overview stats
type AdminStatsResponse struct {
	NewUser          int64 `json:"new_user"`
	NewActiveUser    int64 `json:"new_active_user"`
	NewGalgame       int64 `json:"new_galgame"`
	NewPatchResource int64 `json:"new_patch_resource"`
	NewComment       int64 `json:"new_comment"`
}

// AdminStatsSumResponse is the response for total stats
type AdminStatsSumResponse struct {
	UserCount          int64 `json:"user_count"`
	GalgameCount       int64 `json:"galgame_count"`
	PatchResourceCount int64 `json:"patch_resource_count"`
	PatchCommentCount  int64 `json:"patch_comment_count"`
}
