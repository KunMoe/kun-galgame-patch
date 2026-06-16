package config

import "os"

type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	OAuth        OAuthConfig
	S3           S3Config
	GalgameWiki  GalgameWikiConfig
	ImageService ImageServiceConfig
	CORS         CORSConfig
}

type ServerConfig struct {
	Port string
	Mode string // "dev" or "prod"
}

type DatabaseConfig struct {
	URL             string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int // minutes
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// OAuthConfig holds the OAuth integration settings. ClientID/ClientSecret
// are used both for the user-facing PKCE flow (token exchange / refresh)
// and for service-to-service Basic Auth against /users/batch and /users/search.
type OAuthConfig struct {
	ServerURL    string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// S3Config holds the patch-resource (Backblaze B2) bucket settings. Images
// live in a separate R2 bucket served by image_service — that's
// ImageServiceConfig, not this struct.
//
// Endpoint  : the S3-API origin used by minio-go to PUT/GET/multipart objects
//             (e.g. https://s3.us-east-005.backblazeb2.com).
// PublicURL : the user-facing download prefix that fronts the bucket
//             (e.g. https://oss.moyu.moe). When empty NewS3 falls back to
//             Endpoint + "/" + Bucket — fine for dev (direct B2 download),
//             wrong for prod where B2's egress is metered and we want our
//             own CDN/reverse-proxy domain serving the bytes.
type S3Config struct {
	Endpoint  string
	PublicURL string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// GalgameWikiConfig points at the separately deployed Galgame Wiki Service (D11).
type GalgameWikiConfig struct {
	BaseURL string // e.g. http://127.0.0.1:9280/api
}

// ImageServiceConfig points at the centralized image_service (W2 / PR3b).
// Auth is HTTP Basic with an OAuth client_id/secret (per
// docs/image_service/06-integration-guide.md). ClientID/Secret default to the
// project's OAuth credentials when unset — image_service reuses the OAuth
// `oauth_client` table as its "site" registry, so the same credentials work.
type ImageServiceConfig struct {
	BaseURL      string // e.g. http://127.0.0.1:9278 (no trailing slash)
	CDNBase      string // e.g. http://127.0.0.1:9000/kun-images-dev; serves the ab/cd/<hash>.webp tree
	ClientID     string // defaults to OAuth.ClientID
	ClientSecret string // defaults to OAuth.ClientSecret
}

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
		S3: S3Config{
			// _BUCKET_NAME (not _BUCKET) — keeps the var name in lockstep with
			// legacy next-web and apps/web/.env.production so a single .env
			// file works for both stacks during the migration window.
			Endpoint:  getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_ENDPOINT", ""),
			PublicURL: getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_URL", ""),
			Region:    getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_REGION", ""),
			Bucket:    getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_BUCKET_NAME", ""),
			AccessKey: getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_ACCESS_KEY_ID", ""),
			SecretKey: getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_SECRET_ACCESS_KEY", ""),
		},
		GalgameWiki: GalgameWikiConfig{
			BaseURL: getEnv("KUN_GALGAME_WIKI_BASE_URL", "http://127.0.0.1:9280/api"),
		},
		ImageService: ImageServiceConfig{
			// Fail-fast in prod (audit GPT-L02): these default to localhost dev
			// values, so an unset var in prod would SILENTLY point uploads/CDN
			// at 127.0.0.1 and only fail at runtime. Require them explicitly in
			// prod; keep the dev defaults otherwise.
			BaseURL: getEnvProd("KUN_IMAGE_SERVICE_BASE_URL", "http://127.0.0.1:9278", mode),
			CDNBase: getEnvProd("KUN_IMAGE_CDN_BASE", "http://127.0.0.1:9000/kun-images-dev", mode),
			// Empty fallback → app.go fills from OAuth credentials.
			ClientID:     getEnv("KUN_IMAGE_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("KUN_IMAGE_OAUTH_CLIENT_SECRET", ""),
		},
		CORS: CORSConfig{
			// Default covers both dev frontends: 5213 = legacy next-web
			// (deprecated but kept during transition), 6969 = current
			// Nuxt apps/web. .env / .env.example also list both — keep
			// in sync if a port changes. Without 6969 the browser fails
			// `/auth/me` with "No 'Access-Control-Allow-Origin' header"
			// any time the server boots without .env loaded.
			AllowOrigins: getEnv(
				"CORS_ALLOW_ORIGINS",
				"http://127.0.0.1:5213,http://127.0.0.1:6969",
			),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}

// getEnvProd returns the env var if set; otherwise it panics in prod mode
// (fail-fast — no silent dev-default fallback for things that must be
// configured in production) and returns devFallback in dev.
func getEnvProd(key, devFallback, mode string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if mode == "prod" {
		panic("required environment variable not set in prod mode: " + key)
	}
	return devFallback
}
