// PKCE (Proof Key for Code Exchange) utility for OAuth 2.0 Authorization Code flow

const base64UrlEncode = (input: Uint8Array): string =>
  btoa(String.fromCharCode(...input))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')

const generateCodeVerifier = (): string => {
  const array = new Uint8Array(32)
  crypto.getRandomValues(array)
  return base64UrlEncode(array)
}

const generateCodeChallenge = async (verifier: string): Promise<string> => {
  const encoder = new TextEncoder()
  const data = encoder.encode(verifier)
  const digest = await crypto.subtle.digest('SHA-256', data)
  return base64UrlEncode(new Uint8Array(digest))
}

const generateState = (): string => {
  const array = new Uint8Array(16)
  crypto.getRandomValues(array)
  return Array.from(array, (b) => b.toString(16).padStart(2, '0')).join('')
}

// PKCE `state` + `code_verifier` are stashed in short-lived cookies (NOT
// sessionStorage). The OAuth flow leaves our origin (→ oauth.kungal.com) and
// redirects back to /auth/callback; lightweight / old mobile browsers (Via,
// in-app WebViews) drop the per-tab sessionStorage across that cross-origin
// round-trip, which surfaced as "OAuth callback verification failed". A cookie
// is per-origin (not tab-bound) and survives the round-trip. 10-min TTL
// auto-expires it; SameSite=Lax (it's a first-party cookie read via JS, so Lax
// is enough) + secure on https. Tradeoff vs sessionStorage: cookies are shared
// across tabs, so two concurrent logins in one browser can clobber each other's
// state — rare and acceptable.
const OAUTH_COOKIE_TTL = 600 // seconds (10 min)

const setOAuthCookie = (name: string, value: string): void => {
  const secure = window.location.protocol === 'https:' ? '; secure' : ''
  document.cookie = `${name}=${encodeURIComponent(value)}; max-age=${OAUTH_COOKIE_TTL}; path=/; samesite=lax${secure}`
}

const getOAuthCookie = (name: string): string | null => {
  const prefix = `${name}=`
  for (const part of document.cookie ? document.cookie.split('; ') : []) {
    if (part.startsWith(prefix)) return decodeURIComponent(part.slice(prefix.length))
  }
  return null
}

const deleteOAuthCookie = (name: string): void => {
  document.cookie = `${name}=; max-age=0; path=/; samesite=lax`
}

// Options that tune the authorize redirect for the account-switching flows
// (see docs/oauth/09-account-switching.md):
//   - prompt=select_account → OP renders its account picker (the bag it holds
//     for this browser); paired with login_hint it can switch silently.
//   - prompt=login          → OP forces a fresh credential screen ("add a new
//     account"), and is also what an admin step-up needs.
//   - loginHint=<sub|email> → jump straight to a known account, skipping the
//     picker when the OP bag still has it.
//   - returnTo              → app path to land on after the callback completes
//     (so switching keeps you where you were instead of bouncing to the
//     profile). Stashed in a cookie and consumed by /auth/callback.
interface AuthorizeOptions {
  prompt?: 'login' | 'select_account' | 'none'
  loginHint?: string
  returnTo?: string
}

// Build the standard OAuth authorize URL + cookie-stash PKCE/state.
// Shared between login and register flows — only the OAuth web entry
// point differs (see startOAuthLogin / startOAuthRegister below).
const prepareAuthorizeUrl = async (
  opts: AuthorizeOptions = {}
): Promise<string> => {
  const config = useRuntimeConfig()
  const codeVerifier = generateCodeVerifier()
  const codeChallenge = await generateCodeChallenge(codeVerifier)
  const state = generateState()

  setOAuthCookie('oauth_code_verifier', codeVerifier)
  setOAuthCookie('oauth_state', state)
  if (opts.returnTo) setOAuthCookie('oauth_return_to', opts.returnTo)

  // `/oauth/authorize` is an API endpoint (lives under oauthServerUrl =
  // dev :9277/api/v1, prod oauth.kungal.com/api/v1). The user-facing
  // pages (/auth/register, /forgot, /profile) live on oauthWebUrl
  // (dev :9420, prod oauth.kungal.com) and the API server 302-redirects
  // to those when its consent flow needs UI. Earlier this used oauthWebUrl
  // and produced `:9420/oauth/authorize` which doesn't exist.
  const oauthServerUrl = config.public.oauthServerUrl as string
  const clientId = config.public.oauthClientId as string
  const redirectUri =
    (config.public.oauthRedirectUri as string) ||
    `${window.location.origin}/auth/callback`

  const params = new URLSearchParams({
    client_id: clientId,
    redirect_uri: redirectUri,
    response_type: 'code',
    scope: 'openid profile',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256'
  })
  if (opts.prompt) params.set('prompt', opts.prompt)
  if (opts.loginHint) params.set('login_hint', opts.loginHint)

  return `${oauthServerUrl}/oauth/authorize?${params}`
}

export const startOAuthLogin = async (
  opts: AuthorizeOptions = {}
): Promise<void> => {
  window.location.href = await prepareAuthorizeUrl(opts)
}

// Switch to an account the OP already holds for this browser. login_hint =
// the target's `sub`; OP switches without re-prompting unless the target is an
// admin (step-up → it forces prompt=login itself). If the OP bag no longer has
// the account (logged out elsewhere), it gracefully falls back to login.
// See docs/oauth/09-account-switching.md §3.1 / §3.6.
export const startOAuthSwitchAccount = async (
  loginHint: string,
  returnTo?: string
): Promise<void> => {
  await startOAuthLogin({ prompt: 'select_account', loginHint, returnTo })
}

// Add a brand-new account: force the OP login screen (don't silently re-consent
// the current session). The new account joins the OP bag and our local list.
export const startOAuthAddAccount = async (
  returnTo?: string
): Promise<void> => {
  await startOAuthLogin({ prompt: 'login', returnTo })
}

// Unified-registration entry: bounce the user to OAuth web's /auth/register
// with the full authorize URL stashed as ?redirect=. After registration
// completes, OAuth web auto-logs-in (the Register endpoint issues tokens)
// and window.location.href's to the redirect URL, which restarts the
// standard authorize flow. First-party moyu (auto_consent=true) skips
// the consent UI, code lands on /auth/callback, moyu session created.
// See docs/integration/oauth/05-registration.md.
export const startOAuthRegister = async (): Promise<void> => {
  const config = useRuntimeConfig()
  const oauthWebUrl = config.public.oauthWebUrl as string
  const authorizeUrl = await prepareAuthorizeUrl()
  window.location.href = `${oauthWebUrl}/auth/register?redirect=${encodeURIComponent(authorizeUrl)}`
}

// RP-initiated logout. Clearing only moyu's own session is NOT enough: the
// central OP (oauth.kungal.com) session survives (its localStorage user is
// cross-origin and its refresh cookie is cross-site), so the next login would
// silently re-consent (first-party auto_consent) and log the user straight back
// into the same account. After clearing the local session, callers top-level
// navigate here, which sends the browser to the OP logout entrypoint
// (`{oauthServerUrl}/oauth/logout`, symmetric with /oauth/authorize). The OP
// clears its session and redirects back to `redirect` (validated against this
// client's registered redirect_uris). See docs/oauth/07-logout.md.
export const startOAuthLogout = (): void => {
  const config = useRuntimeConfig()
  const oauthServerUrl = config.public.oauthServerUrl as string
  const clientId = config.public.oauthClientId as string
  const params = new URLSearchParams({
    client_id: clientId,
    redirect: `${window.location.origin}/`
  })
  window.location.href = `${oauthServerUrl}/oauth/logout?${params}`
}

export const verifyOAuthCallback = (): {
  code: string
  codeVerifier: string
} | null => {
  const urlParams = new URLSearchParams(window.location.search)
  const code = urlParams.get('code')
  const returnedState = urlParams.get('state')
  const savedState = getOAuthCookie('oauth_state')

  if (!code || !returnedState || returnedState !== savedState) {
    return null
  }

  const codeVerifier = getOAuthCookie('oauth_code_verifier')
  if (!codeVerifier) {
    return null
  }

  deleteOAuthCookie('oauth_state')
  deleteOAuthCookie('oauth_code_verifier')

  return { code, codeVerifier }
}

// Read (and clear) the post-callback return path stashed by a switch/add flow.
// Returns null when absent or unsafe. Open-redirect guard: only same-origin app
// paths (a single leading slash) are honoured; anything else falls back to the
// caller's default destination.
export const consumeOAuthReturnTo = (): string | null => {
  const value = getOAuthCookie('oauth_return_to')
  if (value) deleteOAuthCookie('oauth_return_to')
  if (value && value.startsWith('/') && !value.startsWith('//')) return value
  return null
}
