import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import type { KunPreferenceDoc } from '~/composables/useKunCloudPreferences'

const { server } = vi.hoisted(() => ({
  server: {
    version: 0,
    doc: null as Partial<KunPreferenceDoc> | null,
    calls: [] as string[],
    // How many of the next PUTs another tab gets in ahead of.
    racing: 0
  }
}))

const ok = <T>(data: T) => ({ code: 0, message: 'OK', data })
const envelope = () =>
  ok({ doc: server.doc, version: server.version, updated_at: null })

mockNuxtImport('useApi', () => () => ({
  get: async (endpoint: string) => {
    if (endpoint !== '/auth/me/preferences') return ok({ blocks: [] })
    server.calls.push('GET')
    return envelope()
  },
  put: async (
    endpoint: string,
    body: { doc: KunPreferenceDoc; version: number }
  ) => {
    if (endpoint !== '/auth/me/preferences') return ok({})
    server.calls.push(`PUT ${body.version}`)
    if (server.racing > 0) {
      server.racing -= 1
      server.version += 1
    }
    if (body.version !== server.version) {
      return { code: 18006, message: 'version conflict', data: null }
    }
    server.version += 1
    server.doc = body.doc
    return envelope()
  }
}))

const DEFAULTS: KunPreferenceDoc = {
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

const stored = (doc: Partial<KunPreferenceDoc>, version = 3) => {
  server.doc = { ...DEFAULTS, ...doc }
  server.version = version
}

// The client plugin is live in this app: signing in pulls, and its watcher on
// the stores schedules every write. Nothing below calls pull or push itself.
const settle = async () => {
  await nextTick()
  await vi.advanceTimersByTimeAsync(1000)
  await new Promise((resolve) => setImmediate(resolve))
}

const signIn = async () => {
  useUserStore().setUser({ id: 7, name: 'kun' })
  await settle()
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  server.version = 0
  server.doc = null
  server.calls.length = 0
  server.racing = 0
  useSettingStore().resetData()
  useUserStore().setMutedMessageTypes([])
})

afterEach(async () => {
  useUserStore().logout()
  await settle()
  vi.useRealTimers()
})

describe('useKunCloudPreferences', () => {
  it('reads on load and writes nothing when a revalidate rewrites equal values', async () => {
    stored({ title_language: 'zh-cn' })
    await signIn()
    expect(useSettingStore().data.titleLanguage).toBe('zh-cn')

    useUserStore().setUser({ id: 7, name: 'kun', bio: 'new bio' })
    useSettingStore().setData({})
    await settle()

    expect(server.calls).toEqual(['GET'])
    expect(server.version).toBe(3)
  })

  it('debounces real changes into one PUT carrying the version it read', async () => {
    stored({})
    await signIn()

    useSettingStore().setData({ showReleaseDate: true })
    await nextTick()
    await vi.advanceTimersByTimeAsync(500)
    useUserStore().toggleMutedMessageType('like')
    await settle()

    expect(server.calls).toEqual(['GET', 'PUT 3'])
    expect(server.doc).toMatchObject({
      show_release_date: true,
      muted_message_types: ['like']
    })

    useSettingStore().setData({ showNsfwBadge: true })
    await nextTick()
    useSettingStore().setData({ showNsfwBadge: false })
    await settle()

    expect(server.calls).toEqual(['GET', 'PUT 3'])
  })

  it('leaves an unborn namespace alone unless this browser was tuned', async () => {
    await signIn()
    expect(server.calls).toEqual(['GET'])

    useUserStore().logout()
    await settle()
    useSettingStore().setData({ galgameListLayout: 'row' })
    await signIn()

    expect(server.calls).toEqual(['GET', 'GET', 'PUT 0'])
    expect(server.doc).toMatchObject({ galgame_list_layout: 'row' })
  })

  it('on a version clash keeps the other tab’s change and writes this one over it once', async () => {
    stored({})
    await signIn()

    stored({ show_nsfw_badge: true }, 4)
    useSettingStore().setData({ showReleaseDate: true })
    await settle()

    expect(server.calls).toEqual(['GET', 'PUT 3', 'GET', 'PUT 4'])
    expect(server.doc).toMatchObject({
      show_nsfw_badge: true,
      show_release_date: true
    })
    expect(useSettingStore().data.showNsfwBadge).toBe(true)
    expect(useSettingStore().data.showReleaseDate).toBe(true)
  })

  it('does not rewrite what the other tab already saved', async () => {
    stored({})
    await signIn()

    stored({ show_release_date: true }, 4)
    useSettingStore().setData({ showReleaseDate: true })
    await settle()

    expect(server.calls).toEqual(['GET', 'PUT 3', 'GET'])
    expect(server.version).toBe(4)
  })

  it('retries a clash once and then stops', async () => {
    stored({})
    await signIn()

    server.racing = 2
    useSettingStore().setData({ showReleaseDate: true })
    await settle()

    expect(server.calls).toEqual(['GET', 'PUT 3', 'GET', 'PUT 4'])
  })
})
