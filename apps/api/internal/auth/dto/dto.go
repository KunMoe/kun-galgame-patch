package dto

// OAuthCallbackRequest is the body of POST /api/v1/auth/oauth/callback.
type OAuthCallbackRequest struct {
	Code         string `json:"code" validate:"required"`
	CodeVerifier string `json:"code_verifier" validate:"required"`
}

// MeResponse is the identity + site-state payload returned by /auth/me and
// the success path of /auth/oauth/callback. It composes:
//
//   - Identity (uid, sub, roles) from the OAuth session / JWT
//   - Display fields (name, avatar, bio) batch-resolved from OAuth /users/batch
//     via pkg/userclient (cached; one network round-trip per logged-in user
//     within the cache TTL)
//   - Site-local fields (moemoepoint, daily counters, follower counts) from
//     the local user row
//
// Composing here means the frontend gets the full profile in one call and
// downstream pages can render KunAvatar / userStore.user without per-page
// userclient.User lookups.
type MeResponse struct {
	UID             int      `json:"uid"`
	Sub             string   `json:"sub"`
	Roles           []string `json:"roles"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	Bio             string   `json:"bio"`
	Moemoepoint     int      `json:"moemoepoint"`
	DailyCheckIn    int      `json:"daily_check_in"`
	DailyImageCount int      `json:"daily_image_count"`
	DailyUploadSize int      `json:"daily_upload_size"`
	FollowerCount   int      `json:"follower_count"`
	FollowingCount  int      `json:"following_count"`
}
