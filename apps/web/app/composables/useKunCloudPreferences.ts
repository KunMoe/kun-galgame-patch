import type { KunSettingData } from '~/stores/settingStore'

// moyu's slot in the account-wide preferences KV. The namespace is moyu's OAuth
// client_id and is chosen by the BFF — the browser never names one, because an
// OAuth token may only touch its own client's namespace anyway.
//
// snake_case keys: this document is read by the account centre's "what does
// this app store about me" view, so it follows the wire convention of every
// other NextMoe payload rather than the store's camelCase.
export interface KunPreferenceDoc {
  galgame_list_layout: KunSettingData['galgameListLayout']
  title_language: KunSettingData['titleLanguage']
  show_japanese_subtitle: boolean
  show_release_date: boolean
  show_nsfw_badge: boolean
  show_galgames_without_resource: boolean
  gallery_sexual_levels: number[]
  gallery_violence_levels: number[]
  muted_message_types: string[]
}

interface PreferenceEnvelope {
  doc: Partial<KunPreferenceDoc> | null
  version: number
  updated_at: string | null
}

// The one code that means "this session cannot reach the KV". It is NOT 40399:
// a settings page writes on every debounce, and turning a missing scope into a
// re-login toast there would fire it over and over. The reader keeps using the
// cookie and never learns anything went wrong, which is the intent.
const PREFERENCES_UNAVAILABLE = 40398
// OAuth's 412 body: someone else's tab wrote this namespace since our read.
const VERSION_CLASH = 18006

const WRITE_DEBOUNCE = 1000

// Module scope, not composable scope: the write-through watcher calls the
// composable afresh on every change, so a timer created inside it would be a
// new timer each time and the debounce would never coalesce anything — one PUT
// per keystroke-equivalent. Only ever touched from the client plugin (the
// server never schedules a write), so the usual "module state leaks across SSR
// requests" hazard does not apply. Same shape as useRefreshMe's bookkeeping.
let pushTimer: ReturnType<typeof setTimeout> | null = null

export const useKunCloudPreferences = () => {
  const settingStore = useSettingStore()
  const userStore = useUserStore()
  const api = useApi()

  const version = useState('kun-cloud-prefs-version', () => -1)
  const disabled = useState('kun-cloud-prefs-disabled', () => false)
  // Applying the cloud document mutates the very stores the write-through
  // watcher listens to. Without this the first sync would immediately push what
  // it had just pulled back up, burning a version on every page load.
  const applying = useState('kun-cloud-prefs-applying', () => false)

  const collect = (): KunPreferenceDoc => ({
    galgame_list_layout: settingStore.data.galgameListLayout ?? 'poster',
    title_language: settingStore.data.titleLanguage ?? 'ja-jp',
    show_japanese_subtitle: settingStore.data.showJapaneseSubtitle ?? false,
    show_release_date: settingStore.data.showReleaseDate ?? false,
    show_nsfw_badge: settingStore.data.showNsfwBadge ?? false,
    show_galgames_without_resource:
      settingStore.data.showGalgamesWithoutResource ?? false,
    gallery_sexual_levels: settingStore.data.gallerySexualLevels ?? [],
    gallery_violence_levels: settingStore.data.galleryViolenceLevels ?? [],
    muted_message_types: userStore.user.muted_message_types ?? []
  })

  const apply = (doc: Partial<KunPreferenceDoc> | null) => {
    if (!doc) return
    applying.value = true
    const next: Partial<KunSettingData> = {}
    if (doc.galgame_list_layout)
      next.galgameListLayout = doc.galgame_list_layout
    if (doc.title_language) next.titleLanguage = doc.title_language
    if (typeof doc.show_japanese_subtitle === 'boolean') {
      next.showJapaneseSubtitle = doc.show_japanese_subtitle
    }
    if (typeof doc.show_release_date === 'boolean') {
      next.showReleaseDate = doc.show_release_date
    }
    if (typeof doc.show_nsfw_badge === 'boolean') {
      next.showNsfwBadge = doc.show_nsfw_badge
    }
    if (typeof doc.show_galgames_without_resource === 'boolean') {
      next.showGalgamesWithoutResource = doc.show_galgames_without_resource
    }
    if (Array.isArray(doc.gallery_sexual_levels)) {
      next.gallerySexualLevels = doc.gallery_sexual_levels
    }
    if (Array.isArray(doc.gallery_violence_levels)) {
      next.galleryViolenceLevels = doc.gallery_violence_levels
    }
    settingStore.setData(next)
    if (Array.isArray(doc.muted_message_types)) {
      userStore.setMutedMessageTypes(doc.muted_message_types)
    }
    nextTick(() => {
      applying.value = false
    })
  }

  const isLocalDefault = (): boolean => {
    const doc = collect()
    return (
      doc.galgame_list_layout === 'poster' &&
      doc.title_language === 'ja-jp' &&
      !doc.show_japanese_subtitle &&
      !doc.show_release_date &&
      !doc.show_nsfw_badge &&
      !doc.show_galgames_without_resource &&
      doc.gallery_sexual_levels.length === 0 &&
      doc.gallery_violence_levels.length === 0 &&
      doc.muted_message_types.length === 0
    )
  }

  const write = async (expected: number | undefined, retry = true) => {
    const res = await api.put<PreferenceEnvelope>('/auth/me/preferences', {
      doc: collect(),
      ...(expected === undefined ? {} : { version: expected })
    })
    if (res.code === 0) {
      version.value = res.data.version
      return
    }
    if (res.code === PREFERENCES_UNAVAILABLE) {
      disabled.value = true
      return
    }
    // 412: another tab (or another device) moved the document under us. The
    // rule is re-read, re-apply, then write once more — not "force it", which
    // would throw away whatever that other tab had just saved.
    if (res.code === VERSION_CLASH && retry) {
      const fresh = await api.get<PreferenceEnvelope>('/auth/me/preferences')
      if (fresh.code !== 0) return
      version.value = fresh.data.version
      apply(fresh.data.doc)
      await write(fresh.data.version, false)
    }
  }

  const pull = async () => {
    if (disabled.value || !userStore.user.id) return
    const res = await api.get<PreferenceEnvelope>('/auth/me/preferences')
    if (res.code === PREFERENCES_UNAVAILABLE) {
      disabled.value = true
      return
    }
    if (res.code !== 0) return

    version.value = res.data.version
    // version 0 is "nobody has ever written this namespace", not an error. It
    // is also the If-Match value that claims the first write — so a reader who
    // already tuned this browser seeds the account from it, and a reader who
    // did not leaves the namespace unborn rather than filling it with defaults.
    if (res.data.version === 0) {
      if (!isLocalDefault()) await write(0)
      return
    }
    apply(res.data.doc)
  }

  const push = () => {
    if (pushTimer) clearTimeout(pushTimer)
    pushTimer = setTimeout(() => {
      pushTimer = null
      if (disabled.value || !userStore.user.id || version.value < 0) return
      write(version.value)
    }, WRITE_DEBOUNCE)
  }

  return { pull, push, collect, applying, disabled, version }
}
