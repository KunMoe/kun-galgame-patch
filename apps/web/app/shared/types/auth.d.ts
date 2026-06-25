// One entry in the OAuth app-directory strip ("可以用这个账号登录以下网站") shown
// on the login modal. Mirrors the moyu backend GET /auth/oauth/ecosystem shape,
// which itself proxies the OAuth provider's public app directory. Display-only
// (no secret / redirect / scope). See docs/oauth/10-app-directory.md.
interface EcosystemApp {
  name: string
  site_domain: string
  logo_url?: string
  tagline?: string
}
