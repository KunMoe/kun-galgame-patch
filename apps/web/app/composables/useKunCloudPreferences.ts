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

const DEFAULT_DOC: KunPreferenceDoc = {
  galgame_list_layout: 'poster',
  title_language: 'ja-jp',
  show_japanese_subtitle: false,
  show_release_date: false,
  show_nsfw_badge: false,
  show_galgames_without_resource: false,
  gallery_sexual_levels: [],
  gallery_violence_levels: [],
  muted_message_types: []
}

const KEYS = Object.keys(DEFAULT_DOC) as (keyof KunPreferenceDoc)[]

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

const sameDoc = (a: KunPreferenceDoc, b: KunPreferenceDoc) =>
  KEYS.every((key) => same(a[key], b[key]))

// A key the stored document lacks, or holds in the wrong shape, keeps
// `fallback`'s value.
const readDoc = (
  doc: Partial<KunPreferenceDoc> | null,
  fallback: KunPreferenceDoc
): KunPreferenceDoc => {
  const out: Record<string, unknown> = { ...fallback }
  for (const key of KEYS) {
    const value: unknown = doc?.[key]
    const like = fallback[key]
    const fits = Array.isArray(like)
      ? Array.isArray(value)
      : typeof value === typeof like && value !== ''
    if (fits) out[key] = value
  }
  return out as unknown as KunPreferenceDoc
}

// Module scope, not composable scope: the write-through watcher calls push on
// every change, so a timer created per composable call would be a new timer
// each time and the debounce would never coalesce anything — one PUT per
// keystroke-equivalent. Only ever touched from the client plugin (the server
// never schedules a write), so the usual "module state leaks across SSR
// requests" hazard does not apply. Same shape as useRefreshMe's bookkeeping.
let pushTimer: ReturnType<typeof setTimeout> | null = null
// Writes run one at a time, so each starts from the version the last one left.
let queue: Promise<void> = Promise.resolve()

export const useKunCloudPreferences = () => {
  const settingStore = useSettingStore()
  const userStore = useUserStore()
  const api = useApi()

  const version = useState('kun-cloud-prefs-version', () => -1)
  const disabled = useState('kun-cloud-prefs-disabled', () => false)
  // The document as of `version`: what this tab last read or wrote. null until
  // the first read lands, and nothing is written before then.
  const base = useState<KunPreferenceDoc | null>(
    'kun-cloud-prefs-base',
    () => null
  )

  const collect = (): KunPreferenceDoc => ({
    galgame_list_layout:
      settingStore.data.galgameListLayout ?? DEFAULT_DOC.galgame_list_layout,
    title_language:
      settingStore.data.titleLanguage ?? DEFAULT_DOC.title_language,
    show_japanese_subtitle:
      settingStore.data.showJapaneseSubtitle ??
      DEFAULT_DOC.show_japanese_subtitle,
    show_release_date:
      settingStore.data.showReleaseDate ?? DEFAULT_DOC.show_release_date,
    show_nsfw_badge:
      settingStore.data.showNsfwBadge ?? DEFAULT_DOC.show_nsfw_badge,
    show_galgames_without_resource:
      settingStore.data.showGalgamesWithoutResource ??
      DEFAULT_DOC.show_galgames_without_resource,
    gallery_sexual_levels: settingStore.data.gallerySexualLevels ?? [],
    gallery_violence_levels: settingStore.data.galleryViolenceLevels ?? [],
    muted_message_types: userStore.user.muted_message_types ?? []
  })

  const apply = (doc: KunPreferenceDoc) => {
    if (sameDoc(doc, collect())) return
    settingStore.setData({
      galgameListLayout: doc.galgame_list_layout,
      titleLanguage: doc.title_language,
      showJapaneseSubtitle: doc.show_japanese_subtitle,
      showReleaseDate: doc.show_release_date,
      showNsfwBadge: doc.show_nsfw_badge,
      showGalgamesWithoutResource: doc.show_galgames_without_resource,
      gallerySexualLevels: doc.gallery_sexual_levels,
      galleryViolenceLevels: doc.gallery_violence_levels
    })
    userStore.setMutedMessageTypes(doc.muted_message_types)
  }

  const adopt = (
    envelope: PreferenceEnvelope,
    local: KunPreferenceDoc
  ): KunPreferenceDoc => {
    version.value = envelope.version
    // version 0 is "nobody has ever written this namespace", not an error. It
    // is also the If-Match value that claims the first write — so a reader who
    // already tuned this browser seeds the account from it, and a reader who
    // did not leaves the namespace unborn rather than filling it with defaults.
    base.value =
      envelope.version === 0 ? DEFAULT_DOC : readDoc(envelope.doc, local)
    return base.value
  }

  const write = async (doc: KunPreferenceDoc, retry: boolean) => {
    const res = await api.put<PreferenceEnvelope>('/auth/me/preferences', {
      doc,
      version: version.value
    })
    if (res.code === 0) {
      version.value = res.data.version
      base.value = doc
      return
    }
    if (res.code === PREFERENCES_UNAVAILABLE) {
      disabled.value = true
      return
    }
    if (res.code !== VERSION_CLASH || !retry) return

    // 412: another tab (or another device) moved the document under us. Keep
    // what they saved, lay over it only the keys this tab changed, and write
    // that once. Re-applying their document wholesale used to discard the very
    // edit that was being saved.
    const fresh = await api.get<PreferenceEnvelope>('/auth/me/preferences')
    if (fresh.code !== 0) return
    const local = collect()
    const changed = KEYS.filter((key) => !same(local[key], base.value?.[key]))
    const theirs = adopt(fresh.data, local)
    const merged: KunPreferenceDoc = {
      ...theirs,
      ...Object.fromEntries(changed.map((key) => [key, local[key]]))
    }
    apply(merged)
    if (!sameDoc(merged, theirs)) await write(merged, false)
  }

  const sync = async () => {
    if (disabled.value || !userStore.user.id || !base.value) return
    const local = collect()
    // Only a value that differs from the last read is a change, never "the
    // watcher fired": setUser replaces the whole user object on every tab
    // revalidate and the deep watch fires on that alone. Writing on it sent
    // 6,612 PUTs against 4,269 GETs in four days, 1,775 of them 412s from tabs
    // racing each other with stale versions (infra oauth logs, 2026-10-19).
    if (sameDoc(local, base.value)) return
    await write(local, true)
  }

  const flush = () => (queue = queue.then(sync))

  const pull = async () => {
    if (disabled.value || !userStore.user.id) return
    const res = await api.get<PreferenceEnvelope>('/auth/me/preferences')
    if (res.code === PREFERENCES_UNAVAILABLE) {
      disabled.value = true
      return
    }
    if (res.code !== 0) return

    const cloud = adopt(res.data, collect())
    if (res.data.version > 0) apply(cloud)
    await flush()
  }

  const push = () => {
    if (pushTimer) clearTimeout(pushTimer)
    pushTimer = setTimeout(() => {
      pushTimer = null
      flush()
    }, WRITE_DEBOUNCE)
  }

  return { pull, push, collect, disabled, version }
}
