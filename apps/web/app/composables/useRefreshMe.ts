import type { UserState } from '~/stores/userStore'

// useRefreshMe — the "revalidate" half of stale-while-revalidate for the
// current-user snapshot.
//
// The user store is a cookie-persisted snapshot (instant SSR render, no
// flicker — see stores/userStore.ts). That snapshot is the "stale" half: it
// can lag for days because the only built-in refresh was User.vue's onMounted,
// which fires once per FULL page load and never on SPA navigation. So an avatar
// / 用户名 / 签名 changed elsewhere (the OAuth profile page, the forum) didn't
// show up in an already-open moyu tab until a hard refresh.
//
// This re-pulls the whole MeResponse from /auth/me and merges it into the
// store, so avatar AND name AND bio all refresh together — not just one field.
// It's deduped by a staleTime window (and an in-flight guard) so passive
// triggers (tab focus, network reconnect) can fire freely without spamming the
// endpoint. This is exactly what TanStack Query / Pinia Colada do internally
// (staleTime + refetchOnWindowFocus); we keep the persisted store rather than
// pull in a whole data-fetching layer for a single object.
//
// Auth-expiry (40100 / 40101) logout is handled centrally in
// useApi.handleAuthExpiry on every request, so we only handle the happy path.

// Passive triggers within this window of the last refresh are skipped.
const STALE_TIME = 60_000

// Module-level SWR bookkeeping. Only ever touched on the client (every entry
// point guards on import.meta.client), so the usual "module state leaks across
// SSR requests" hazard does not apply — the server never writes these.
let lastRefreshAt = 0
let inflight: Promise<void> | null = null

export const useRefreshMe = () => {
  const userStore = useUserStore()
  const api = useApi()
  const { rememberUser } = useKnownAccounts()

  const refreshMe = async (): Promise<void> => {
    if (!import.meta.client) return
    if (!userStore.user.id) return
    if (inflight) return inflight
    if (Date.now() - lastRefreshAt < STALE_TIME) return

    inflight = (async () => {
      const res = await api.get<UserState>('/auth/me')
      // Guard on a non-empty name: when OAuth /users/batch is momentarily down,
      // composeMe (auth/handler.go) still returns code:0 but with BLANK display
      // fields (name/avatar/bio = ""). Merging those would wipe the good cached
      // identity — and since isLoggedIn requires a non-empty name, the top bar
      // would flicker to a logged-out look on a transient upstream blip. A real
      // user always has a name (OAuth requires it), so an empty name means
      // "degraded response": skip the merge and keep the existing snapshot. The
      // session is still valid (id present), so we deliberately do NOT log out.
      // Mirrors rememberUser's own `if (!user.name) return` guard.
      if (res.code === 0 && res.data?.name) {
        // setUser merges into existing state and preserves muted_message_types.
        userStore.setUser(res.data)
        // Keep this account fresh in the local switch list (account switching §3.6).
        rememberUser(userStore.user)
      }
    })().finally(() => {
      // Stamp even on failure so a transient error doesn't turn into a retry
      // storm; the next trigger after STALE_TIME tries again.
      lastRefreshAt = Date.now()
      inflight = null
    })
    return inflight
  }

  return { refreshMe }
}
