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
  const baseUrl = config.public.apiBase || 'http://127.0.0.1:5214/api/v1'

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

  const request = async <T>(
    endpoint: string,
    options: ApiOptions = {}
  ): Promise<ApiResponse<T>> => {
    const { method = 'GET', body, headers = {} } = options

    try {
      const res = await $fetch<ApiResponse<T>>(`${baseUrl}${endpoint}`, {
        method,
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
    delete: <T>(endpoint: string) => request<T>(endpoint, { method: 'DELETE' }),
    patch: <T>(endpoint: string, body?: Record<string, unknown>) =>
      request<T>(endpoint, { method: 'PATCH', body })
  }
}
