import { defineStore } from 'pinia'

// How this browser wants adult content shown. Three states, not two:
//   'hide' — NSFW games never leave the API (content_limit=sfw)
//   'blur' — they come back and render masked, revealed per item on click
//   'show' — they come back and render plainly
// Only the first bit reaches the wire: stanceToContentLimit folds blur and show
// together, because masking is presentation and the opt-in is the same one.
//
// This is the ANONYMOUS store. A logged-in reader's stance lives on the NextMoe
// account (userStore.adult_confirmed + nsfw_display) and this cookie is not
// mirrored from it — otherwise logging out would leave the account's stance
// behind on a shared browser.
export type KunNsfwStance = 'hide' | 'blur' | 'show'

export interface KunSettingData {
  kunNsfwEnable: KunNsfwStance
  // Per-patch NSFW acknowledgements for anonymous callers.
  //
  // Background: anonymous + 'sfw' callers get a 404 from the game page when the
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
  // Anonymous-only state: a logged-in reader is gated by their account stance
  // instead. Clearing it on logout would be incorrect (an anonymous browser
  // that logged in then out should keep its prior NSFW acks).
  nsfwAckedIds: number[]

  // ── Galgame card display preferences (the /galgame "显示设置" panel) ──
  // Cookie-persisted with the rest of the store so SSR renders cards in the
  // chosen language on first paint (no hydration flash). Read with a `?? default`
  // guard at the use site — an older cookie won't carry these keys.
  //
  // Shape every galgame list draws: 'poster' (default) is the grid of covers,
  // 'row' is the wide two-column row that also carries the credits and tags.
  galgameListLayout: 'poster' | 'row'
  // Preferred language for game titles site-wide: 'ja-jp' (default) or 'zh-cn'.
  titleLanguage: 'zh-cn' | 'ja-jp'
  // Show the Japanese title as a subtitle under the title. Default off.
  showJapaneseSubtitle: boolean
  // Show the game's release date on the card. Default off.
  showReleaseDate: boolean
  // Show the game's NSFW / age-rating badge on the card. Default off.
  showNsfwBadge: boolean
  // Include galgames that have no patch resources (resource_count = 0). Default
  // off → lists only show games with patches. Unlike the other four (pure card
  // rendering), this drives a backend filter applied to EVERY moyu galgame list
  // (home / galgame / ranking / a user's patches / favorites / contributions):
  // useApi forwards it as the global `include_empty` query param, so the rows +
  // pagination total stay correct. Wiki-backed lists (tag / search) are exempt.
  showGalgamesWithoutResource: boolean

  // ── Galgame 画廊 (screenshot) per-rating filter — see components/galgame/
  // Gallery.vue. Two independent axes, each a persisted set of opted-in levels:
  //   色情 (sexual): the global NSFW mode reveals every level; in SFW mode only
  //     safe (0) + the levels listed here show. A shot nobody assessed counts
  //     as level 1.
  //   暴力 (violence): ALWAYS an explicit per-level opt-in (the NSFW mode does
  //     NOT unlock it), gated behind a confirm. Default empty = hidden.
  gallerySexualLevels: number[]
  galleryViolenceLevels: number[]
}

const initialState: KunSettingData = {
  kunNsfwEnable: 'hide',
  nsfwAckedIds: [],
  galgameListLayout: 'poster',
  titleLanguage: 'ja-jp',
  showJapaneseSubtitle: false,
  showReleaseDate: false,
  showNsfwBadge: false,
  showGalgamesWithoutResource: false,
  gallerySexualLevels: [],
  galleryViolenceLevels: []
}

export const useSettingStore = defineStore('setting', {
  state: (): { data: KunSettingData } => ({
    data: { ...initialState }
  }),
  actions: {
    setData(data: Partial<KunSettingData>) {
      this.data = { ...this.data, ...data }
    },
    setNsfwStance(v: KunNsfwStance) {
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
    // Toggle a gallery rating level on/off (cookie-persisted). Replace-array
    // (not push) so pinia tracks the mutation and the persist plugin writes it
    // through; `?? []` guards a legacy cookie missing the key.
    toggleGalleryLevel(axis: 'sexual' | 'violence', level: number) {
      const key =
        axis === 'sexual' ? 'gallerySexualLevels' : 'galleryViolenceLevels'
      const cur = this.data[key] ?? []
      this.data[key] = cur.includes(level)
        ? cur.filter((l) => l !== level)
        : [...cur, level]
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
    // These options replace the global cookieOptions rather than merging,
    // which is why sameSite is repeated here.
    storage: piniaPluginPersistedstate.cookies({
      maxAge: 60 * 60 * 24 * 400,
      sameSite: 'lax'
    }),
    // Narrowing the type above does nothing to a cookie already holding an old
    // string, and every reader of this field would then fall through to a label
    // that resolves to undefined and renders blank — a mode no button can
    // leave. So rewrite the value in place on hydrate, exactly as the removal of
    // the 'nsfw' (NSFW-only) mode already had to.
    //
    // 'all' → 'show', not 'blur': those readers had opted in to seeing NSFW and
    // a silent downgrade to masked covers would read as a site regression.
    // Anything unrecognised lands on 'hide', the safe default.
    afterHydrate: ({ store }) => {
      const data = (store as unknown as { data: KunSettingData }).data
      const legacy: Record<string, KunNsfwStance> = {
        sfw: 'hide',
        all: 'show',
        nsfw: 'show'
      }
      const current = data.kunNsfwEnable as string
      if (current === 'hide' || current === 'blur' || current === 'show') return
      data.kunNsfwEnable = legacy[current] ?? 'hide'
      store.$persist()
    }
  }
})
