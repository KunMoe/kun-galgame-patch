// Backward-compat alias for the LEGACY Hikari partner URL.
//
// The legacy Next.js API served partners at `/api/hikari`; moyu's Go API serves
// the same endpoint at `/api/v1/hikari`. In prod, Traefik routes `/api/v1/*` to
// the Go API and everything else (including this legacy `/api/hikari`) to the
// Nuxt server — so without this alias the old URL 404s on Nuxt, breaking every
// partner (kungal, touchgal, shionlib, hikarinagi, …) that still calls it.
//
// We PROXY (not 301-redirect) so a partner's cross-origin request gets a single
// response carrying the Go API's `Access-Control-Allow-Origin` header. h3's
// proxyRequest forwards the inbound `Origin` request header (so the Go API's
// HikariCORS allowlist can match it) and copies the upstream response headers
// back (so ACAO reaches the browser). It also preserves the status/body, so the
// legacy {success,message,data} envelope and the 400/404 cases pass through
// verbatim. All methods are proxied — the OPTIONS preflight is answered by the
// Go API's HikariCORS (204).
export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event)
  // Server-side hop: prefer the in-container base (apiBaseSsr → the moyu-api
  // service name in docker), fall back to the public base for local `air` dev
  // where apiBaseSsr is empty.
  const base =
    (config.apiBaseSsr as string) ||
    (config.public.apiBase as string) ||
    'http://127.0.0.1:5214/api/v1'
  // Preserve the original query string (?vndb_id=...).
  const search = getRequestURL(event).search
  return proxyRequest(event, `${base}/hikari${search}`)
})
