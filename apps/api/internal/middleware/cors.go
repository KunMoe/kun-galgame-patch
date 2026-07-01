package middleware

import (
	"regexp"
	"strings"

	"kun-galgame-patch-api/pkg/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

func CORS(cfg config.CORSConfig) fiber.Handler {
	return cors.New(cors.Config{
		// /api/v1/hikari is a public external API with its OWN partner-domain
		// allowlist (HikariCORS) — skip the app-frontend CORS for it so the
		// route owns the preflight + Access-Control-Allow-Origin (this CORS
		// allows only moyu's own origins, which would reject every partner).
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/v1/hikari")
		},
		AllowOrigins:     splitOrigins(cfg.AllowOrigins),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}

// splitOrigins turns the comma-separated CORS_ALLOW_ORIGINS env value into the
// []string that Fiber v3's cors.Config expects, trimming incidental whitespace
// (v3 matches origins exactly, so a stray space would silently break CORS).
func splitOrigins(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// hikariOriginPatterns is the partner-site allowlist for the external Hikari
// API, ported 1:1 from the legacy next-api/hikari route. The ([\w-]+\.)*
// prefix allows wildcard subdomains (e.g. cdn.shionlib.com).
var hikariOriginPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^http://localhost:\d+$`),
	regexp.MustCompile(`^http://127\.0\.0\.1:\d+$`),
	regexp.MustCompile(`^https://([\w-]+\.)*himoe\.uk$`),
	regexp.MustCompile(`^https://([\w-]+\.)*hikarinagi\.com$`),
	regexp.MustCompile(`^https://([\w-]+\.)*hikarinagi\.org$`),
	regexp.MustCompile(`^https://([\w-]+\.)*shionlib\.com$`),
	regexp.MustCompile(`^https://([\w-]+\.)*touchgal\.us$`),
	regexp.MustCompile(`^https://([\w-]+\.)*touchgal\.top$`),
	regexp.MustCompile(`^https://([\w-]+\.)*touchgal\.ink$`),
	regexp.MustCompile(`^https://([\w-]+\.)*nyne\.dev$`),
	regexp.MustCompile(`^https://([\w-]+\.)*kungal\.com$`),
	regexp.MustCompile(`^https://([\w-]+\.)*kungal\.org$`),
	regexp.MustCompile(`^https://([\w-]+\.)*lycorisgal\.com$`),
	regexp.MustCompile(`^https://([\w-]+\.)*galgamex\.net$`),
	regexp.MustCompile(`^https://([\w-]+\.)*galgamex\.top$`),
	regexp.MustCompile(`^https://([\w-]+\.)*galgamex\.com$`),
	regexp.MustCompile(`^https://([\w-]+\.)*sharotto\.com$`),
	regexp.MustCompile(`^https://([\w-]+\.)*kisuacg\.moe$`),
}

func hikariOriginAllowed(origin string) bool {
	for _, re := range hikariOriginPatterns {
		if re.MatchString(origin) {
			return true
		}
	}
	return false
}

// HikariCORS is the dynamic CORS for the external Hikari API: it reflects the
// request Origin ONLY when it matches a partner-site pattern, and answers the
// OPTIONS preflight. Public read API → no credentials. Applied via api.Use so
// it runs for both GET and the OPTIONS preflight.
func HikariCORS() fiber.Handler {
	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin != "" && hikariOriginAllowed(origin) {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Vary", "Origin")
			c.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Set("Access-Control-Max-Age", "86400")
		}
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}
