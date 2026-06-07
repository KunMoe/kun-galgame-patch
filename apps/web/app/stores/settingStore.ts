import { defineStore } from 'pinia'

// NSFW preference. Maps 1:1 to the wiki content_limit query parameter per
// docs/galgame_wiki/00-handbook §16 — useApi forwards it verbatim:
//   'sfw'  — only SFW games (the safe-by-default; also what we send on
//             SSR fallback / signed-out / cookie missing)
//   'nsfw' — only NSFW games (curiosity mode)
//   'all'  — both
//
// Stored as a plain string (not boolean) so the front-end NSFW toggle UI
// can offer all three modes if needed without rewriting the protocol.
export type KunNsfwPreference = 'sfw' | 'nsfw' | 'all'

export interface KunSettingData {
  kunNsfwEnable: KunNsfwPreference
  // Per-patch NSFW acknowledgements for anonymous callers.
  //
  // Background: anonymous + 'sfw' callers get a 404 from /patch/:id when the
  // patch is NSFW (SEO safe-by-default). Product rule: such users should be
  // able to opt-in *per patch* via a "this game contains NSFW, click to
  // continue" confirm — without flipping the global NSFW mode.
  //
  // When the user clicks confirm on a NSFW patch detail, we push that
  // patch.id into this array (cookie-persisted). useApi reads route.params.id
  // and if the route's id is in here, it sends content_limit=all for the
  // current request — exactly that patch (and its sub-endpoints sharing
  // :id) becomes visible, others stay gated.
  //
  // Logged-in users bypass this entirely via useApi's userStore.id > 0
  // check, so this array is essentially anonymous-only state; clearing it
  // on logout would be incorrect (an anonymous browser that logged in then
  // out should keep its prior NSFW acks).
  nsfwAckedIds: number[]

  // ── Galgame card display preferences (the /galgame "显示设置" panel) ──
  // Cookie-persisted with the rest of the store so SSR renders cards in the
  // chosen language on first paint (no hydration flash). Read with a `?? default`
  // guard at the use site — an older cookie won't carry these keys.
  //
  // Preferred language for the game TITLE on cards: 'zh-cn' (default) or 'ja-jp'.
  titleLanguage: 'zh-cn' | 'ja-jp'
  // Show the Japanese title as a subtitle under the title. Default off.
  showJapaneseSubtitle: boolean
  // Show the game's release date on the card. Default off.
  showReleaseDate: boolean
  // Show the game's NSFW / age-rating badge on the card. Default on.
  showNsfwBadge: boolean
  // Include galgames that have no patch resources (resource_count = 0). Default
  // off → lists only show games with patches. Unlike the other four (pure card
  // rendering), this drives a backend filter applied to EVERY moyu galgame list
  // (home / galgame / ranking / a user's patches / favorites / contributions):
  // useApi forwards it as the global `include_empty` query param, so the rows +
  // pagination total stay correct. Wiki-backed lists (tag / search) are exempt.
  showGalgamesWithoutResource: boolean
}

const initialState: KunSettingData = {
  kunNsfwEnable: 'sfw',
  nsfwAckedIds: [],
  titleLanguage: 'zh-cn',
  showJapaneseSubtitle: false,
  showReleaseDate: false,
  showNsfwBadge: true,
  showGalgamesWithoutResource: false
}

export const useSettingStore = defineStore('setting', {
  state: (): { data: KunSettingData } => ({
    data: { ...initialState }
  }),
  actions: {
    setData(data: Partial<KunSettingData>) {
      this.data = { ...this.data, ...data }
    },
    setNsfwPreference(v: KunNsfwPreference) {
      this.data.kunNsfwEnable = v
    },
    ackNsfw(id: number) {
      // `?? []` guards a null/legacy cookie value — isNsfwAcked runs in useApi
      // during SSR for anonymous detail-route requests, so an unguarded
      // .includes() here would 500 the page.
      const ids = this.data.nsfwAckedIds ?? []
      if (id > 0 && !ids.includes(id)) {
        // Replace the array (don't .push) so pinia's reactivity tracks the
        // mutation and the cookie-persist plugin writes the new value.
        this.data.nsfwAckedIds = [...ids, id]
      }
    },
    isNsfwAcked(id: number): boolean {
      return id > 0 && (this.data.nsfwAckedIds ?? []).includes(id)
    },
    resetData() {
      this.data = { ...initialState }
    }
  },
  // Cookie-backed (NOT localStorage) so SSR can read the preference from the
  // incoming request and bake the right content_limit query into the very
  // first wiki call. Without this, an anonymous crawler / first-paint pass
  // would always render the sfw fallback for one frame before the client
  // hydrates and switches — but more importantly, *we want sfw on SSR for
  // signed-out callers anyway*, so the only practical effect is "the user's
  // own opt-in survives a hard refresh".
  persist: {
    key: 'kun-patch-setting-store',
    storage: piniaPluginPersistedstate.cookies()
  }
})
