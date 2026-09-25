// The persisted cookies are only written when a store changes, so their
// Max-Age counted from the last change and an anonymous reader's NSFW choice
// expired however often they came back. $persist() rewrites the cookie even
// when nothing changed (verified in a production build; `refresh: true` is not
// needed), which is what renews it here.
const hasCookie = (name: string) =>
  document.cookie.split('; ').some((entry) => entry.startsWith(`${name}=`))

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook('app:mounted', () => {
    nuxtApp.runWithContext(() => {
      if (hasCookie('kun-patch-setting-store')) useSettingStore().$persist()
      if (hasCookie('kun-patch-user-store')) useUserStore().$persist()
    })
  })
})
