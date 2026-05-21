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
	About        AboutConfig
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

type S3Config struct {
	Endpoint  string
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
	CDNBase      string // e.g. http://127.0.0.1:9000/kun-images-dev; serves the /img/ab/cd/<hash>.webp tree
	ClientID     string // defaults to OAuth.ClientID
	ClientSecret string // defaults to OAuth.ClientSecret
}

type CORSConfig struct {
	AllowOrigins string
}

// AboutConfig points at the directory holding the static .mdx posts that drive
// the /about pages. In dev this is typically ../web/posts; in prod the build
// pipeline copies the same tree next to the binary.
type AboutConfig struct {
	PostsDir string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("KUN_SERVER_PORT", "5214"),
			Mode: getEnv("KUN_SERVER_MODE", "dev"),
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
			Endpoint:  getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_ENDPOINT", ""),
			Region:    getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_REGION", ""),
			Bucket:    getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_BUCKET", ""),
			AccessKey: getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_ACCESS_KEY_ID", ""),
			SecretKey: getEnv("KUN_VISUAL_NOVEL_S3_STORAGE_SECRET_ACCESS_KEY", ""),
		},
		GalgameWiki: GalgameWikiConfig{
			BaseURL: getEnv("KUN_GALGAME_WIKI_BASE_URL", "http://127.0.0.1:9280/api"),
		},
		ImageService: ImageServiceConfig{
			BaseURL: getEnv("KUN_IMAGE_SERVICE_BASE_URL", "http://127.0.0.1:9278"),
			CDNBase: getEnv("KUN_IMAGE_CDN_BASE", "http://127.0.0.1:9000/kun-images-dev"),
			// Empty fallback → app.go fills from OAuth credentials.
			ClientID:     getEnv("KUN_IMAGE_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("KUN_IMAGE_OAUTH_CLIENT_SECRET", ""),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv("CORS_ALLOW_ORIGINS", "http://127.0.0.1:5213"),
		},
		About: AboutConfig{
			PostsDir: getEnv("KUN_POSTS_DIR", "../web/posts"),
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
