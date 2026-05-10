package dto

// Username / bio / password / email / avatar mutations were removed from
// this site -- they are owned by OAuth (PATCH /auth/me on the OAuth server).

type GetUserProfileRequest struct {
	Page  int `query:"page" validate:"min=1"`
	Limit int `query:"limit" validate:"min=1,max=20"`
}

type SearchUserRequest struct {
	Query string `query:"query" validate:"required,min=1,max=20"`
}

type UserInfoResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
	Bio            string `json:"bio"`
	Moemoepoint    int    `json:"moemoepoint"`
	Role           int    `json:"role"`
	FollowerCount  int    `json:"follower_count"`
	FollowingCount int    `json:"following_count"`
	RegisterTime   string `json:"register_time"`
	PatchCount     int64  `json:"patch_count"`
	ResourceCount  int64  `json:"resource_count"`
	CommentCount   int64  `json:"comment_count"`
	FavoriteCount  int64  `json:"favorite_count"`
	IsFollowed     bool   `json:"is_followed"`
}
