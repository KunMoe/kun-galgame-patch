package config

import (
	"os"
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
// Nothing in /api/v1 needs it -- the frontend prepends its own domains, so the
// API returns bare keys and hashes. The developer-platform face cannot: a third
// party has no way to know that a patch id becomes www.moyu.moe/patch/<id>, so
// every row it answers carries an absolute web_url built from this.
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

type TrustConfig struct {
	BaseURL        string
	Site           string
	CallbackSecret string
}

// DlsiteConfig is the DLsite purchase entry on a game page.
//
// LinkTemplate is a whole template, not assembled parts: DLsite's affiliate path
// differs per site segment, so a path change stays an env edit. Empty template
// AND empty StoreAPIKey = no purchase entry is rendered at all.
//
// StoreAPIKey reaches infra's /v2/store face, which mints the per-site short
// link that carries click attribution. It is a SECOND developer key, separate
// from the catalog one: the face is gated on the scope store:read and the v2
// limiter buckets per key, so minting links must not spend the catalogue's
// minute budget. The aliases belong to the OAuth client rather than the key, so
// moyu and kungal stay separately attributed even sharing an affiliate account.
type DlsiteConfig struct {
	LinkTemplate string
	CouponURL    string
	StoreAPIBase string
	StoreAPIKey  string
}

func (c DlsiteConfig) Configured() bool { return c.LinkTemplate != "" }

func (c DlsiteConfig) StoreConfigured() bool { return c.StoreAPIBase != "" && c.StoreAPIKey != "" }

type CORSConfig struct {
	AllowOrigins string
}

func Load() *Config {
	mode := getEnv("KUN_SERVER_MODE", "dev")
	return &Config{
		Server: ServerConfig{
			Port: getEnv("KUN_SERVER_PORT", "5214"),
			Mode: mode,
		},
		Database: DatabaseConfig{
			URL:             mustGetEnv("KUN_DATABASE_URL"),
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: 60,
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "127.0.0.1"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		OAuth: OAuthConfig{
			ServerURL:    getEnv("OAUTH_SERVER_URL", "http://127.0.0.1:9277/api/v1"),
			ClientID:     getEnv("OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("OAUTH_CLIENT_SECRET", ""),
			RedirectURI:  getEnv("OAUTH_REDIRECT_URI", ""),
		},
		NextMoeAPI: NextMoeAPIConfig{
			BaseURL: getEnv("KUN_NEXTMOE_API_BASE", "http://127.0.0.1:19281"),
			APIKey:  getEnv("KUN_NEXTMOE_API_KEY", ""),
		},
		ImageService: ImageServiceConfig{
			BaseURL:      getEnvProd("KUN_IMAGE_SERVICE_BASE_URL", "http://127.0.0.1:9278", mode),
			CDNBase:      getEnvProd("KUN_IMAGE_CDN_BASE", "http://127.0.0.1:9000/kun-images-dev", mode),
			ClientID:     getEnv("KUN_IMAGE_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("KUN_IMAGE_OAUTH_CLIENT_SECRET", ""),
		},
		Artifact: ArtifactConfig{
			BaseURL:      getEnvProd("KUN_ARTIFACT_SERVICE_BASE_URL", "http://127.0.0.1:9279", mode),
			ClientID:     getEnv("KUN_ARTIFACT_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("KUN_ARTIFACT_OAUTH_CLIENT_SECRET", ""),
		},
		Trust: TrustConfig{
			BaseURL:        getEnvOptionalProd("KUN_TRUST_BASE_URL", "http://127.0.0.1:9283", mode),
			Site:           getEnv("KUN_TRUST_SITE", "moyu"),
			CallbackSecret: getEnv("KUN_TRUST_CALLBACK_SECRET", ""),
		},
		Dlsite: DlsiteConfig{
			LinkTemplate: getEnv("KUN_DLSITE_LINK_TEMPLATE", ""),
			CouponURL:    getEnv("KUN_DLSITE_COUPON_URL", ""),
			StoreAPIBase: getEnv("KUN_STORE_API_BASE", getEnv("KUN_NEXTMOE_API_BASE", "http://127.0.0.1:19281")),
			StoreAPIKey:  getEnv("KUN_STORE_API_KEY", ""),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv(
				"CORS_ALLOW_ORIGINS",
				"http://127.0.0.1:5213,http://127.0.0.1:6969",
			),
		},
		Site: SiteConfig{
			BaseURL: strings.TrimRight(getEnv("KUN_SITE_BASE_URL", "https://www.moyu.moe"), "/"),
		},
		BotSubmit: BotSubmit{
			Key:    getEnv("KUN_BOT_SUBMIT_KEY", ""),
			UserID: int(getEnvUint64("KUN_BOT_SUBMIT_USER_ID", 0)),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvUint64(key string, fallback uint64) uint64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}

func getEnvProd(key, devFallback, mode string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if mode == "prod" {
		panic("required environment variable not set in prod mode: " + key)
	}
	return devFallback
}

func getEnvOptionalProd(key, devFallback, mode string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if mode == "prod" {
		return ""
	}
	return devFallback
}
