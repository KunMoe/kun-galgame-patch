import { describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// Each SSR request gets its own pinia. A state factory that hands every one
// of them the same nested arrays lets persistedstate's deep watcher in every
// request subscribe to those arrays, and nothing ever unsubscribes it.
const twoRequests = <T>(use: () => T): [T, T] => {
  setActivePinia(createPinia())
  const a = use()
  setActivePinia(createPinia())
  return [a, use()]
}

describe('store initial state is not shared between pinia instances', () => {
  it('settingStore', () => {
    const [a, b] = twoRequests(useSettingStore)
    for (const key of [
      'nsfwAckedIds',
      'gallerySexualLevels',
      'galleryViolenceLevels'
    ] as const) {
      expect(a.$state.data[key]).not.toBe(b.$state.data[key])
    }
  })

  it('userStore', () => {
    const [a, b] = twoRequests(useUserStore)
    for (const key of ['roles', 'site_roles', 'muted_message_types'] as const) {
      expect(a.$state.user[key]).not.toBe(b.$state.user[key])
    }
  })
})
