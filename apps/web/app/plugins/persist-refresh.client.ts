// The persisted store cookies (kun-patch-user-store / kun-patch-setting-store)
// carry a Max-Age, but pinia-plugin-persistedstate only rewrites a cookie when
// its store mutates. A user who sets a preference and then changes nothing else
// would lose it when the Max-Age lapses, and the next load would fall back to
// the 'sfw' default. Re-persisting once per load makes the window slide, so a
// kept session keeps its preferences.
//
// Only cookies that already exist are refreshed: an anonymous visitor who never
// touched a setting still gets no cookie. `$persist()` writes the state the
// store already hydrated from that cookie, so the value does not change.
//
// Resolved in app:mounted (not at plugin init) so this carries no ordering
// dependency on Pinia being installed first — the same reason
// revalidate-me.client.ts resolves its composable inside runWithContext.
const hasCookie = (key: string): boolean =>
  document.cookie.split('; ').some((entry) => entry.startsWith(`${key}=`))

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook('app:mounted', () => {
    nuxtApp.runWithContext(() => {
      const persisted = [
        { key: 'kun-patch-user-store', store: useUserStore() },
        { key: 'kun-patch-setting-store', store: useSettingStore() }
      ]
      for (const { key, store } of persisted) {
        if (hasCookie(key)) store.$persist()
      }
    })
  })
})
