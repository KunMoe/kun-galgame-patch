package middleware

import "testing"

// TestHikariOriginAllowed locks the external Hikari API's partner-domain
// allowlist: legitimate partner origins (incl. wildcard subdomains) pass, and
// look-alike / prefix / scheme-downgrade spoofs are rejected.
func TestHikariOriginAllowed(t *testing.T) {
	allow := []string{
		"http://localhost:3000",
		"http://127.0.0.1:6969",
		"https://himoe.uk",
		"https://hikarinagi.com",
		"https://www.hikarinagi.com",
		"https://hikarinagi.org",
		"https://cdn.shionlib.com",
		"https://touchgal.us",
		"https://touchgal.top",
		"https://touchgal.ink",
		"https://nyne.dev",
		"https://kungal.com",
		"https://kungal.org",
		"https://lycorisgal.com",
		"https://galgamex.net",
		"https://galgamex.top",
		"https://galgamex.com",
		"https://sharotto.com",
		"https://kisuacg.moe",
		"https://www.kisuacg.moe",
	}
	for _, o := range allow {
		if !hikariOriginAllowed(o) {
			t.Errorf("expected ALLOWED: %s", o)
		}
	}

	deny := []string{
		"",
		"https://evil.com",
		"https://hikarinagi.com.evil.com", // suffix spoof
		"https://evilhikarinagi.com",      // prefix spoof
		"https://nothikarinagi.com",
		"https://shionlib.com.attacker.net",
		"http://hikarinagi.com", // http not allowed for partners (only localhost)
		"https://hikarinagi.dev",
		"https://kungal.com.cn",
	}
	for _, o := range deny {
		if hikariOriginAllowed(o) {
			t.Errorf("expected DENIED: %s", o)
		}
	}
}
