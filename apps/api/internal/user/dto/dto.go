package dto

import (
	"kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/pkg/userclient"
)

// The profile tabs page at 24, the same as every other galgame card grid; a 20
// ceiling answered "Limit length must not be greater than 20" instead.
type GetUserProfileRequest struct {
	Page  int `query:"page" validate:"min=1"`
	Limit int `query:"limit" validate:"min=1,max=24"`
}

type GetFollowListRequest struct {
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=24"`
}

type FollowListResponse struct {
	Items      []model.UserFollowItem `json:"items"`
	NextCursor string                 `json:"next_cursor,omitempty"`
	Total      int64                  `json:"total"`
}

type SearchUserRequest struct {
	Query string `query:"query" validate:"required,min=1,max=20"`
}

type UserInfoResponse struct {
	ID             int                   `json:"id"`
	Name           string                `json:"name"`
	Avatar         string                `json:"avatar"`
	Cosmetics      *userclient.Cosmetics `json:"cosmetics,omitempty"`
	Bio            string                `json:"bio"`
	Roles          []string              `json:"roles"`
	SiteRoles      []string              `json:"site_roles"`
	Moemoepoint    int                   `json:"moemoepoint"`
	FollowerCount  *int                  `json:"follower_count,omitempty"`
	FollowingCount *int                  `json:"following_count,omitempty"`
	RegisterTime   string                `json:"register_time"`
	PatchCount     int64                 `json:"patch_count"`
	ResourceCount  int64                 `json:"resource_count"`
	CommentCount   int64                 `json:"comment_count"`
	FavoriteCount  int64                 `json:"favorite_count"`
	IsFollowed     bool                  `json:"is_followed"`
}
