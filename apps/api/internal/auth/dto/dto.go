package dto

// OAuthCallbackRequest is the body of POST /api/v1/auth/oauth/callback.
type OAuthCallbackRequest struct {
	Code         string `json:"code" validate:"required"`
	CodeVerifier string `json:"code_verifier" validate:"required"`
}

// MeResponse is the minimal identity payload returned by /auth/me and the
// success path of /auth/oauth/callback.
//
// Display fields (name, avatar, bio) are NOT in this struct -- they live on
// OAuth and the frontend pulls them via /oauth/userinfo or /users/batch.
// Phase 5-6 will compose local fields (moemoepoint, daily counters, follow
// counts) into this payload via the userclient.
type MeResponse struct {
	UID   int      `json:"uid"`
	Sub   string   `json:"sub"`
	Roles []string `json:"roles"`
}
