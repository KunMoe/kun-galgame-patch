// A page load is covered by useRefreshMe; this covers a login or an account
// switch without one, and a logout must not leave the account's list in a
// cookie on a shared browser.
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook('app:mounted', () => {
    const userStore = useUserStore()
    watch(
      () => userStore.user.id,
      (id) => {
        if (id) {
          nuxtApp.runWithContext(() => useTagBlocks().refresh())
          return
        }
        useTagBlockStore().$patch({ accountUid: 0, account: emptyTagBlocks() })
      }
    )
  })
})
