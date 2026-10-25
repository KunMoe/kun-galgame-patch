package middleware

import (
	"regexp"
	"strings"

	"kun-galgame-patch-api/internal/face"
	"kun-galgame-patch-api/pkg/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

func CORS(cfg config.CORSConfig) fiber.Handler {
	return cors.New(cors.Config{
		// Two paths bring their own policy. /api/v1/hikari answers a partner
		// allowlist by regex; the developer-platform face answers any origin
		// and sends no credentials, which this policy cannot express at the
		// same time.
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/v1/hikari") || face.IsPath(c.Path())
		},
		AllowOrigins:     splitOrigins(cfg.AllowOrigins),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"X-Content-Truncated"},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}

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
