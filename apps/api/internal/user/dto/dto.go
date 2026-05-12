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

// UserInfoResponse composes site-local fields (moemoepoint, follower/following
// counts, content counts) with display fields (name/avatar/bio) batch-resolved
// from OAuth /users/batch.
//
// Role is no longer present -- per-site roles were retired in the OAuth
// migration. Frontend should read roles from the JWT roles claim returned by
// /auth/me when it needs to render role badges.
type UserInfoResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
	Bio            string `json:"bio"`
	Moemoepoint    int    `json:"moemoepoint"`
	FollowerCount  int    `json:"follower_count"`
	FollowingCount int    `json:"following_count"`
	RegisterTime   string `json:"register_time"`
	PatchCount     int64  `json:"patch_count"`
	ResourceCount  int64  `json:"resource_count"`
	CommentCount   int64  `json:"comment_count"`
	FavoriteCount  int64  `json:"favorite_count"`
	IsFollowed     bool   `json:"is_followed"`
}
