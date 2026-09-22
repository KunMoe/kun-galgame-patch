// Cloud preference sync for the signed-in reader: the display settings this
// site used to keep only in a cookie now live in the NextMoe account, so a
// second device starts where the first one left off.
//
// Client-only by construction. The cookie is still the store an anonymous
// reader has, and still the offline cache for everyone — the cloud copy layers
// on top of it rather than replacing it, which is why a session that cannot
// reach the KV (no `preferences` scope yet) keeps working with no visible
// difference.
//
// Everything hangs off app:mounted, for two reasons: the stores are certain to
// exist by then whatever the plugin order turns out to be, and applying a cloud
// document mid-hydration would rewrite markup the server had already sent.
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook('app:mounted', () => {
    const userStore = useUserStore()
    const settingStore = useSettingStore()

    let watching = false

    const startWriteThrough = () => {
      if (watching) return
      watching = true

      watch(
        () => [
          settingStore.data.galgameListLayout,
          settingStore.data.titleLanguage,
          settingStore.data.showJapaneseSubtitle,
          settingStore.data.showReleaseDate,
          settingStore.data.showNsfwBadge,
          settingStore.data.showGalgamesWithoutResource,
          settingStore.data.gallerySexualLevels,
          settingStore.data.galleryViolenceLevels,
          userStore.user.muted_message_types
        ],
        () => {
          nuxtApp.runWithContext(() => {
            const { push, applying } = useKunCloudPreferences()
            if (applying.value) return
            push()
          })
        },
        { deep: true }
      )
    }

    // Fires for an already-signed-in reader and again the moment a login lands,
    // which is the only other point at which a session can gain the scope this
    // needs.
    watch(
      () => userStore.user.id,
      (id) => {
        if (!id) return
        nuxtApp.runWithContext(async () => {
          await useKunCloudPreferences().pull()
          startWriteThrough()
        })
      },
      { immediate: true }
    )
  })
})
