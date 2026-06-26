import { useEventListener } from '@vueuse/core'

// SWR "revalidate" triggers for the current-user snapshot (avatar / 用户名 /
// 签名 / 萌萌点). Re-pulls /auth/me when:
//   - the tab regains visibility (you changed your profile elsewhere — the
//     OAuth site / forum — and switched back to moyu), and
//   - the network reconnects.
// Deduped by useRefreshMe's staleTime window, so rapid tab-switching is cheap.
//
// This is the focus/reconnect half of stale-while-revalidate; the cookie-
// persisted user store is the instant-render "stale" half. Mirrors TanStack
// Query / Pinia Colada — and like React Query v5 we key off `visibilitychange`
// (not `focus`), which avoids spurious refetches from window-chrome focus.
//
// runWithContext keeps the Nuxt context valid across the event-driven async
// continuation. We resolve useRefreshMe() *inside* it (not at plugin-init) for
// two reasons: (1) the handlers fire from raw DOM events with no active context,
// so useApi()/useUserStore() inside refreshMe need it restored; (2) it avoids
// any plugin-ordering dependency on Pinia being installed before this plugin.
export default defineNuxtPlugin((nuxtApp) => {
  const trigger = () =>
    nuxtApp.runWithContext(() => useRefreshMe().refreshMe())

  useEventListener(document, 'visibilitychange', () => {
    if (document.visibilityState === 'visible') trigger()
  })
  useEventListener(window, 'online', trigger)
})
