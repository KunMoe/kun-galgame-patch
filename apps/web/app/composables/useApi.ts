// useApi is a thin wrapper over $fetch that always targets the Go API base.
//
// The backend uses a server-side Redis session keyed by the `kun_session`
// httpOnly cookie (see apps/api/internal/middleware/auth.go). There is no
// client-managed access_token — we rely on `credentials: 'include'` so the
// browser attaches the session cookie automatically. OAuth token refresh is
// performed by the server in the background when the upstream token nears
// expiry, so the client has no refresh endpoint to call.
//
// On 401/auth-expired, we surface the error response to the caller; the
// caller (typically a page or store) decides whether to redirect to login.
interface ApiOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  body?: Record<string, unknown>
  headers?: Record<string, string>
}

interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

interface ApiError {
  code: number
  message: string
}

export const useApi = () => {
  const config = useRuntimeConfig()
  // Dual base: SSR (in-container) uses the docker service URL (apiBaseSsr);
  // the browser uses the host-port public URL. apiBaseSsr is empty outside
  // docker → falls back to public.apiBase.
  const baseUrl =
    (import.meta.server && config.apiBaseSsr
      ? (config.apiBaseSsr as string)
      : config.public.apiBase) || 'http://127.0.0.1:5214/api/v1'

  // `credentials: 'include'` only attaches the session cookie in the BROWSER.
  // During SSR (Nuxt server) there is no cookie jar, so an auth-gated
  // useAsyncData fetch would 401 and the page would hydrate from an empty
  // payload — making logged-in content (messages, chat rooms, ...) render
  // then "disappear" on refresh. Capture the incoming request's Cookie
  // header at setup time and forward it on server-side requests so SSR and
  // CSR agree. Must be read here (composable/setup scope), not inside the
  // async closure.
  const ssrCookie = import.meta.server
    ? useRequestHeaders(['cookie']).cookie
    : undefined

  // NSFW preference is encoded as the `content_limit` query parameter on
  // every request, per docs/galgame_wiki/00-handbook §16. We append it
  // here (not via header) because wiki's spec explicitly forbids custom
  // headers / JSON-as-header for the NSFW gate. Endpoints that don't care
  // about content_limit ignore the extra query — Fiber doesn't 400 on
  // unknown params, and the moyu backend only reads it where applicable.
  //
  // Resolution priority (first match wins):
  //   1. Explicit cookie preference != 'sfw' — user picked nsfw / all in
  //      the top-bar switcher, honour it verbatim. This is the ONLY way
  //      listing pages (home / galgame / resource / ranking / user-tabs /
  //      taxonomy) ever return NSFW content; just being logged-in does
  //      NOT flip lists to all — per product rule, "页面上的各种游戏列表
  //      只有用户打开显示全部内容才会显示". The logged-in convenience
  //      only applies to (2) below — directly opening a NSFW detail URL.
  //   2. Detail-page routes (/patch/<id>(/...)? or /resource/<id>) AND
  //      (logged-in OR per-patch ack present) → 'all'. Logged-in users
  //      who land on a NSFW patch's detail URL see the content directly,
  //      no confirm step. Anonymous + ack'd is the "I already confirmed"
  //      branch from pages/patch/[id].vue.
  //   3. Default 'sfw' — SEO safe-by-default for anonymous crawlers, and
  //      the default for every list/index/tool surface regardless of
  //      login state.
  //
  // The detail-route regex is intentionally narrow: only patch/resource
  // detail pages get the bypass. /tag/<id>, /official/<id>, /user/<id> +
  // their child tabs are list-semantics (show many galgames under a
  // tag/user) and stay sfw-default — wiki's own /tag/:name endpoint
  // already filters server-side per §16.2.
  //
  // Captured at setup time so a single request closure sees one snapshot;
  // toggling NSFW mode after the request has started doesn't retroactively
  // mutate the in-flight URL. The NSFWSwitcher / confirm flow both
  // location.reload() to make new state take effect.
  const setting = useSettingStore()
  const userStore = useUserStore()
  const route = useRoute()

  const isDetailRoute = /^\/(patch|resource)\/\d+/.test(route.path)

  const contentLimit = (() => {
    if (setting.data.kunNsfwEnable !== 'sfw') return setting.data.kunNsfwEnable
    if (isDetailRoute) {
      if (userStore.user.id > 0) return 'all'
      const routeId = Number(route.params.id)
      if (routeId > 0 && setting.isNsfwAcked(routeId)) return 'all'
    }
    return 'sfw'
  })()

  const appendContentLimit = (endpoint: string): string => {
    if (!contentLimit) return endpoint
    const sep = endpoint.includes('?') ? '&' : '?'
    return `${endpoint}${sep}content_limit=${contentLimit}`
  }

  // `include_empty`: the "显示设置 → 显示无补丁资源的游戏" display pref
  // (settingStore). Appended GLOBALLY — like content_limit — so every moyu
  // galgame-list surface (home / galgame / ranking / user patches / favorites /
  // contributions) honors it; non-list endpoints ignore the extra query.
  //
  // Read LIVE here (NOT snapshotted like contentLimit above) on purpose: the
  // 显示设置 panel lives only on /galgame, and toggling it there calls refresh()
  // — reading the store at request time lets that refetch pick up the new value
  // without the full location.reload() the NSFW switcher needs. Other pages
  // remount on navigation and read the current value at mount anyway.
  //
  // Only sent when enabled: when off (the default) we omit it, and each backend
  // list endpoint defaults to hiding resource-less games (utils.IncludeEmptyGalgames).
  const appendIncludeEmpty = (endpoint: string): string => {
    if (!setting.data.showGalgamesWithoutResource) return endpoint
    const sep = endpoint.includes('?') ? '&' : '?'
    return `${endpoint}${sep}include_empty=true`
  }

  // Centralized auto-logout on a definitively-unauthenticated response.
  //
  // Without this the ONLY thing that reacts to a dead server-side session is
  // User.vue's onMounted /auth/me check — which runs once per full page load,
  // NOT on SPA navigation (the top bar lives in the layout and never
  // re-mounts). So a session that expires while the tab stays open would leave
  // the top bar / admin gates / message bell showing the old identity until the
  // next hard refresh, while every auth-gated action silently 401s. Reacting
  // here means ANY request that comes back unauthenticated immediately wipes
  // the persisted identity, so the whole UI flips to logged-out at once.
  //
  // Policy mirrors User.vue EXACTLY: only 40100 (no/expired cookie) and 40101
  // (session dead) wipe the store. Any OTHER non-zero code (5xx, network blip,
  // transient OAuth refresh failure that KEEPS the cookie for the next-request
  // retry) must NOT log out — that was the "登录之后过一会自动退出" bug. Caveat:
  // the backend also returns 40101 on a transient refresh failure during the
  // hard-expiry window; we accept that rare spurious logout to keep the
  // contract identical to User.vue rather than guess from the client.
  //
  // Client-only (store mutation + DOM toast must not run during SSR, where they
  // would desync hydration), and a no-op once already logged out — so a
  // concurrent fan-out of 401s fires exactly one logout + one toast.
  const handleAuthExpiry = (code: number) => {
    if (!import.meta.client) return
    if (code !== 40100 && code !== 40101) return
    if (userStore.user.id <= 0) return
    userStore.logout()
    useKunMessage('登录状态已失效，请重新登录', 'warn')
  }

  const request = async <T>(
    endpoint: string,
    options: ApiOptions = {}
  ): Promise<ApiResponse<T>> => {
    const { method = 'GET', body, headers = {} } = options
    const url = `${baseUrl}${appendIncludeEmpty(appendContentLimit(endpoint))}`

    try {
      const res = await $fetch<ApiResponse<T>>(url, {
        method,
        // SSR-only ceiling. A stalled server-side $fetch with no timeout never
        // settles → the awaiting Nitro render hangs forever, pinning its whole
        // render context in the heap and leaking the in/out sockets. Over days
        // these accumulate until the event loop GC-thrashes at 100% CPU.
        // import.meta.server is a build-time constant, so the client bundle
        // keeps its no-timeout behavior (long uploads etc. are unaffected).
        timeout: import.meta.server ? 10000 : undefined,
        body: body ? JSON.stringify(body) : undefined,
        headers: {
          'Content-Type': 'application/json',
          ...(ssrCookie ? { cookie: ssrCookie } : {}),
          ...headers
        },
        credentials: 'include'
      })
      // OAuth code 10014 = account banned. Per docs/oauth/api-reference.md
      // the frontend must NOT redirect to /login (re-login hits the same
      // error). Send the user to the dedicated banned-account page instead.
      if (res.code === 10014 && import.meta.client) {
        navigateTo({
          path: '/account-banned',
          query: res.message ? { reason: res.message } : {}
        })
      }
      handleAuthExpiry(res.code)
      return res
    } catch (error: unknown) {
      const fetchError = error as { statusCode?: number; data?: ApiError }
      const code = fetchError.data?.code ?? fetchError.statusCode ?? -1
      if (code === 10014 && import.meta.client) {
        navigateTo({
          path: '/account-banned',
          query: fetchError.data?.message
            ? { reason: fetchError.data.message }
            : {}
        })
      }
      handleAuthExpiry(code)
      return {
        code,
        message: fetchError.data?.message ?? 'Request failed',
        data: null as T
      }
    }
  }

  return {
    get: <T>(endpoint: string) => request<T>(endpoint, { method: 'GET' }),
    post: <T>(endpoint: string, body?: Record<string, unknown>) =>
      request<T>(endpoint, { method: 'POST', body }),
    put: <T>(endpoint: string, body?: Record<string, unknown>) =>
      request<T>(endpoint, { method: 'PUT', body }),
    // Some Wiki proxy endpoints (DELETE /galgame/:gid/links|/aliases) take a
    // JSON `{ id }` body — see docs/galgame_wiki/03-relations.md. body is
    // optional so existing body-less callers are unaffected.
    delete: <T>(endpoint: string, body?: Record<string, unknown>) =>
      request<T>(endpoint, { method: 'DELETE', body }),
    patch: <T>(endpoint: string, body?: Record<string, unknown>) =>
      request<T>(endpoint, { method: 'PATCH', body })
  }
}
