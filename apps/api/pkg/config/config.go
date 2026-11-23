package config

import (
	"strconv"
	"strings"
)

type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	OAuth        OAuthConfig
	NextMoeAPI   NextMoeAPIConfig
	ImageService ImageServiceConfig
	Artifact     ArtifactConfig
	Trust        TrustConfig
	Community    CommunityConfig
	Chat         ChatConfig
	ActivityPush ActivityPushConfig
	Dlsite       DlsiteConfig
	CORS         CORSConfig
	Site         SiteConfig
	BotSubmit    BotSubmit
}

type BotSubmit struct {
	Key    string
	UserID int
}

func (b BotSubmit) Configured() bool { return b.Key != "" && b.UserID > 0 }

// SiteConfig is what this service knows about its own public address.
//
// /api/v1 answers do not use it -- the frontend prepends its own domains, so the
// API returns bare keys and hashes. The developer-platform face cannot: a third
// party has no way to know that a patch id becomes www.moyu.moe/patch/<id>, so
// every row it answers carries an absolute web_url built from this. The trust
// subject-kind registration builds its callback URL from it too.
type SiteConfig struct {
	BaseURL string
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	URL             string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type OAuthConfig struct {
	ServerURL    string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type NextMoeAPIConfig struct {
	BaseURL string
	APIKey  string
}

type ImageServiceConfig struct {
	BaseURL      string
	CDNBase      string
	ClientID     string
	ClientSecret string
}

type ArtifactConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
}

// CommunityConfig points at the NextMoe community primitive, which owns every
// comment wall on this site. Image, artifact and community authenticate moyu by
// its own OAuth client, so their ClientID/ClientSecret are always the OAuth pair.
type CommunityConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
}

func (c CommunityConfig) Configured() bool {
	return c.BaseURL != "" && c.ClientID != "" && c.ClientSecret != ""
}

// ChatConfig points at NextMoe's chat service, which holds every direct message;
// /api/v1/im relays to it under the session's own token.
//
// Enabled is the cutover and flips together with the web's
// NUXT_PUBLIC_CHAT_ENABLED: once direct messages are served from chat, the old
// PRIVATE rooms here refuse writes, because infra's import only sweeps what was
// written before the switch.
type ChatConfig struct {
	BaseURL string
	Enabled bool
}

// ActivityPushConfig switches on the push of this site's activity into the
// network-wide following feed. An https OAuth redirect is not enough on its
// own: a staging deployment is https too and may be bound to a site in the
// production community, which is how letmoe-staging threads reached it.
type ActivityPushConfig struct {
	Enabled bool
}

type TrustConfig struct {
	BaseURL        string
	Site           string
	CallbackURL    string
	CallbackSecret string
}

// DlsiteConfig reaches infra's /v2/store face, which mints the per-site short
// link that carries click attribution. StoreAPIKey is a SECOND developer key,
// separate from the catalog one: the face is gated on the scope store:read and
// the v2 limiter buckets per key, so minting links must not spend the
// catalogue's minute budget. The aliases belong to the OAuth client rather than
// the key, so moyu and kungal stay separately attributed even sharing an
// affiliate account. The affiliate template and the coupon page are site
// settings, not configuration.
type DlsiteConfig struct {
	StoreAPIBase string
	StoreAPIKey  string
}

func (c DlsiteConfig) StoreConfigured() bool { return c.StoreAPIBase != "" && c.StoreAPIKey != "" }

type CORSConfig struct {
	AllowOrigins string
}

// Load takes every default from the deployment it runs in: KUN_SERVER_MODE=prod
// picks production's topology on dokploy-network, anything else the local dev
// stack. The production compose file therefore carries only the mode, the
// bootstrap pair and what has no safe default; a value equal to its default
// does not belong there.
func Load() *Config {
	if err := loadStore(); err != nil {
		panic(err.Error())
	}
	dbURL := lookupEnvOnly(DatabaseURLEnv)
	if dbURL == "" {
		panic("required environment variable not set: " + DatabaseURLEnv)
	}
	mode := env("KUN_SERVER_MODE", "dev")
	byMode := func(dev, prod string) string {
		if mode == "prod" {
			return prod
		}
		return dev
	}
	oauthID := env("OAUTH_CLIENT_ID", "df3ff6008d740bfacbe46aa8cf483cf2")
	oauthSecret := envRequiredInProd("OAUTH_CLIENT_SECRET", mode)
	nextMoeBase := env("KUN_NEXTMOE_API_BASE", byMode("http://127.0.0.1:9281", "http://catalog:9281"))
	return &Config{
		Server: ServerConfig{
			Port: env("KUN_SERVER_PORT", "5214"),
			Mode: mode,
		},
		Database: DatabaseConfig{
			URL:             dbURL,
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: 60,
		},
		Redis: RedisConfig{
			Host:     env("REDIS_HOST", byMode("127.0.0.1", "redis")),
			Port:     env("REDIS_PORT", "6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       0,
		},
		OAuth: OAuthConfig{
			ServerURL:    env("OAUTH_SERVER_URL", byMode("http://127.0.0.1:9277/api/v1", "http://oauth:9277/api/v1")),
			ClientID:     oauthID,
			ClientSecret: oauthSecret,
			RedirectURI:  env("OAUTH_REDIRECT_URI", byMode("http://127.0.0.1:6969/auth/callback", "https://www.moyu.moe/auth/callback")),
		},
		NextMoeAPI: NextMoeAPIConfig{
			BaseURL: nextMoeBase,
			APIKey:  env("KUN_NEXTMOE_API_KEY", ""),
		},
		ImageService: ImageServiceConfig{
			BaseURL: env("KUN_IMAGE_SERVICE_BASE_URL", byMode("http://127.0.0.1:9278", "http://image:9278")),
			// Must equal infra's KUN_IMAGE_PUBLIC_BASE_URL, or the backend and the
			// frontend emit different URLs for the same hash.
			CDNBase:      env("KUN_IMAGE_CDN_BASE", byMode("http://127.0.0.1:9290", "https://image.kungal.iloveren.link")),
			ClientID:     oauthID,
			ClientSecret: oauthSecret,
		},
		Artifact: ArtifactConfig{
			BaseURL:      env("KUN_ARTIFACT_SERVICE_BASE_URL", byMode("http://127.0.0.1:9279", "http://artifact:9279")),
			ClientID:     oauthID,
			ClientSecret: oauthSecret,
		},
		Trust: TrustConfig{
			BaseURL: env("KUN_TRUST_BASE_URL", byMode("http://127.0.0.1:9283", "http://trust:9283")),
			Site:    env("KUN_TRUST_SITE", "moyu"),
			// The enforcement callback goes over the internal alias, like kungal's
			// kungal-api:2334, so Cloudflare's WAF never sees it. Not `web`: infra's
			// web answers to that alias too and the requests would alternate.
			CallbackURL:    env("KUN_TRUST_CALLBACK_URL", byMode("", "http://moyu-api:5214/api/v1/trust/callback")),
			CallbackSecret: env("KUN_TRUST_CALLBACK_SECRET", ""),
		},
		Community: CommunityConfig{
			BaseURL:      env("KUN_COMMUNITY_API_BASE", byMode("http://127.0.0.1:9282/api/v1/community", "http://community:9282/api/v1/community")),
			ClientID:     oauthID,
			ClientSecret: oauthSecret,
		},
		Chat: ChatConfig{
			BaseURL: env("KUN_CHAT_API_BASE", byMode("http://127.0.0.1:9285", "http://chat:9285")),
			Enabled: envBool("KUN_CHAT_ENABLED", mode == "prod"),
		},
		ActivityPush: ActivityPushConfig{
			Enabled: envBool("KUN_ACTIVITY_PUSH_ENABLED", false),
		},
		Dlsite: DlsiteConfig{
			StoreAPIBase: env("KUN_STORE_API_BASE", nextMoeBase),
			StoreAPIKey:  env("KUN_STORE_API_KEY", ""),
		},
		CORS: CORSConfig{
			AllowOrigins: env(
				"CORS_ALLOW_ORIGINS",
				byMode("http://127.0.0.1:5213,http://127.0.0.1:6969", "https://www.moyu.moe,https://moyu.moe"),
			),
		},
		Site: SiteConfig{
			BaseURL: strings.TrimRight(env("KUN_SITE_BASE_URL", "https://www.moyu.moe"), "/"),
		},
		BotSubmit: BotSubmit{
			Key:    env("KUN_BOT_SUBMIT_KEY", ""),
			UserID: envInt("KUN_BOT_SUBMIT_USER_ID", byMode("", "125528")),
		},
	}
}

func env(key, fallback string) string { return lookup(key, fallback) }

func envBool(key string, fallback bool) bool {
	if v, err := strconv.ParseBool(lookup(key, strconv.FormatBool(fallback))); err == nil {
		return v
	}
	return fallback
}

func envInt(key, fallback string) int {
	n, err := strconv.Atoi(lookup(key, fallback))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// envRequiredInProd fails the boot in prod when neither the environment nor
// the store has the value.
func envRequiredInProd(key, mode string) string {
	v := lookup(key, "")
	if v == "" && mode == "prod" {
		panic("required variable not set in prod mode (environment and config store are both empty): " + key)
	}
	return v
}
