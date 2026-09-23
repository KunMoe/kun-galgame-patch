package dto

import "kun-galgame-patch-api/pkg/userclient"

type OAuthCallbackRequest struct {
	Code         string `json:"code" validate:"required"`
	CodeVerifier string `json:"code_verifier" validate:"required"`
}

type MeResponse struct {
	ID              int      `json:"id"`
	Sub             string   `json:"sub"`
	Roles           []string `json:"roles"`
	SiteRoles       []string `json:"site_roles"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	AvatarImageHash string   `json:"avatar_image_hash"`
	// Never omitempty: setUser spreads this over the cookie-persisted store, so
	// an absent key would keep a frame the user has already taken off.
	Cosmetics       *userclient.Cosmetics `json:"cosmetics"`
	Bio             string                `json:"bio"`
	Moemoepoint     int                   `json:"moemoepoint"`
	DailyCheckIn    int                   `json:"daily_check_in"`
	DailyImageCount int                   `json:"daily_image_count"`
	DailyUploadSize int64                 `json:"daily_upload_size"`
	FollowerCount   int                   `json:"follower_count"`
	FollowingCount  int                   `json:"following_count"`

	// The account's stored content stance, verbatim from OAuth. It is NOT the
	// effective one: the client must fold it as
	// `adult_confirmed ? nsfw_display : 'hide'`. NsfwDisplay is omitempty and an
	// absent key means "we could not read it this time" — the client keeps the
	// stance it already had rather than falling back and flipping the reader.
	AdultConfirmed bool   `json:"adult_confirmed"`
	NsfwDisplay    string `json:"nsfw_display,omitempty"`
}
