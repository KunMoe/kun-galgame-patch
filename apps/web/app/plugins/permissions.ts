import { useEventListener } from '@vueuse/core'

const STALE_TIME = 60_000

// Fetched during SSR so a gated page (the admin panel, its menu) renders the
// same on the server as on the client.
export default defineNuxtPlugin(async (nuxtApp) => {
  if (import.meta.server) {
    await usePermissions().refresh()
    return
  }

  let lastRefreshAt = Date.now()
  const refresh = () => {
    lastRefreshAt = Date.now()
    return nuxtApp.runWithContext(() => usePermissions().refresh())
  }

  nuxtApp.hook('app:mounted', () => {
    const userStore = useUserStore()
    watch(() => userStore.user.id, refresh)
    useEventListener(document, 'visibilitychange', () => {
      if (
        document.visibilityState === 'visible' &&
        Date.now() - lastRefreshAt > STALE_TIME
      ) {
        refresh()
      }
    })
  })
})
