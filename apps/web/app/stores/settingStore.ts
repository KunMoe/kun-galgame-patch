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
}

const initialState: KunSettingData = {
  kunNsfwEnable: 'sfw'
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
