package config

import "testing"

// The values docker-compose.prod.yml set before 2026-11-23, when the lines
// moved into the prod defaults. A drift here changes production silently.
func TestProdDefaultsAreTheDeployment(t *testing.T) {
	for _, key := range []string{
		KEKEnv, "KUN_SERVER_PORT", "REDIS_HOST", "REDIS_PORT", "OAUTH_SERVER_URL", "OAUTH_CLIENT_ID",
		"OAUTH_REDIRECT_URI", "KUN_NEXTMOE_API_BASE", "KUN_STORE_API_BASE", "KUN_IMAGE_SERVICE_BASE_URL",
		"KUN_IMAGE_CDN_BASE", "KUN_ARTIFACT_SERVICE_BASE_URL", "KUN_TRUST_BASE_URL", "KUN_TRUST_SITE",
		"KUN_TRUST_CALLBACK_URL", "KUN_COMMUNITY_API_BASE", "KUN_CHAT_API_BASE", "KUN_CHAT_ENABLED",
		"KUN_ACTIVITY_PUSH_ENABLED", "CORS_ALLOW_ORIGINS", "KUN_SITE_BASE_URL", "KUN_BOT_SUBMIT_USER_ID",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("KUN_SERVER_MODE", "prod")
	t.Setenv(DatabaseURLEnv, "postgresql://unused")
	t.Setenv("OAUTH_CLIENT_SECRET", "s")

	cfg := Load()
	for name, got := range map[string]any{
		"port":           cfg.Server.Port,
		"redis":          cfg.Redis.Host + ":" + cfg.Redis.Port,
		"oauth server":   cfg.OAuth.ServerURL,
		"oauth client":   cfg.OAuth.ClientID,
		"oauth redirect": cfg.OAuth.RedirectURI,
		"catalog":        cfg.NextMoeAPI.BaseURL,
		"store":          cfg.Dlsite.StoreAPIBase,
		"image":          cfg.ImageService.BaseURL,
		"image cdn":      cfg.ImageService.CDNBase,
		"artifact":       cfg.Artifact.BaseURL,
		"trust":          cfg.Trust.BaseURL,
		"trust site":     cfg.Trust.Site,
		"trust callback": cfg.Trust.CallbackURL,
		"community":      cfg.Community.BaseURL,
		"chat":           cfg.Chat.BaseURL,
		"chat enabled":   cfg.Chat.Enabled,
		"activity push":  cfg.ActivityPush.Enabled,
		"cors":           cfg.CORS.AllowOrigins,
		"site":           cfg.Site.BaseURL,
		"bot user":       cfg.BotSubmit.UserID,
		"image creds":    cfg.ImageService.ClientID + "/" + cfg.ImageService.ClientSecret,
	} {
		want := map[string]any{
			"port":           "5214",
			"redis":          "redis:6379",
			"oauth server":   "http://oauth:9277/api/v1",
			"oauth client":   "df3ff6008d740bfacbe46aa8cf483cf2",
			"oauth redirect": "https://www.moyu.moe/auth/callback",
			"catalog":        "http://catalog:9281",
			"store":          "http://catalog:9281",
			"image":          "http://image:9278",
			"image cdn":      "https://image.kungal.iloveren.link",
			"artifact":       "http://artifact:9279",
			"trust":          "http://trust:9283",
			"trust site":     "moyu",
			"trust callback": "http://moyu-api:5214/api/v1/trust/callback",
			"community":      "http://community:9282/api/v1/community",
			"chat":           "http://chat:9285",
			"chat enabled":   true,
			"activity push":  false,
			"cors":           "https://www.moyu.moe,https://moyu.moe",
			"site":           "https://www.moyu.moe",
			"bot user":       125528,
			"image creds":    "df3ff6008d740bfacbe46aa8cf483cf2/s",
		}[name]
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}
