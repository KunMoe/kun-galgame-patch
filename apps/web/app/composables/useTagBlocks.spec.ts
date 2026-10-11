import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

const { calls, messages } = vi.hoisted(() => ({
  calls: [] as string[],
  messages: [] as string[]
}))

const ok = <T>(data: T) => ({ code: 0, message: 'OK', data })

mockNuxtImport('useApi', () => () => ({
  get: async (endpoint: string) => {
    calls.push(`GET ${endpoint}`)
    return ok({
      blocks: [
        { tag_id: 6, mode: 'tree', created_at: '', updated_at: '' },
        { tag_id: 9, mode: 'exact', created_at: '', updated_at: '' }
      ]
    })
  },
  put: async (endpoint: string, body: { mode: string }) => {
    calls.push(`PUT ${endpoint} ${body.mode}`)
    return ok({})
  },
  delete: async (endpoint: string) => {
    calls.push(`DELETE ${endpoint}`)
    return ok({ deleted: true })
  }
}))

mockNuxtImport('useKunMessage', () => (message: string) => {
  messages.push(message)
})

const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

const blockCalls = () => calls.filter((call) => call.includes('/tag-block'))

const signIn = (id = 7) => useUserStore().setUser({ id, name: 'kun' })

afterEach(() => {
  calls.length = 0
  messages.length = 0
  useUserStore().logout()
  useTagBlockStore().$reset()
})

describe('useTagBlocks', () => {
  it('keeps a signed-out reader in this browser and sends ids sorted', async () => {
    const tb = useTagBlocks()
    await tb.block(9)
    await tb.block(3, 'exact')
    await tb.block(5)
    expect(calls).toEqual([])
    expect(tb.blockKey.value).toBe('exclude_tag_trees=5,9&exclude_tag_ids=3')

    await tb.block(9, 'exact')
    await tb.unblock(3)
    expect(tb.modeOf(9)).toBe('exact')
    expect(tb.blockKey.value).toBe('exclude_tag_trees=5&exclude_tag_ids=9')
  })

  it("a login loads the account's list, and the browser's is not applied", async () => {
    useTagBlockStore().anonymous = { tree: [1], exact: [] }
    const tb = useTagBlocks()
    signIn()
    expect(tb.blockKey.value).toBe('')

    await settle()
    expect(blockCalls()).toEqual(['GET /tag-block'])
    expect(tb.blockKey.value).toBe('exclude_tag_trees=6&exclude_tag_ids=9')

    await tb.unblock(6)
    expect(blockCalls()).toEqual(['GET /tag-block', 'DELETE /tag-block/6'])
    expect(tb.blockKey.value).toBe('exclude_tag_ids=9')
    expect(useTagBlockStore().anonymous).toEqual({ tree: [1], exact: [] })
  })

  it('loads the account list before its first write instead of overwriting it', async () => {
    signIn()
    await settle()
    useTagBlockStore().$patch({ accountUid: 0 })
    calls.length = 0

    const tb = useTagBlocks()
    await tb.block(12)
    expect(blockCalls()).toEqual(['GET /tag-block', 'PUT /tag-block/12 tree'])
    expect(tb.blockKey.value).toBe('exclude_tag_trees=6,12&exclude_tag_ids=9')
  })

  it("never applies another account's mirror", async () => {
    const store = useTagBlockStore()
    store.$patch({ accountUid: 8, account: { tree: [42], exact: [] } })
    const tb = useTagBlocks()
    signIn(7)
    expect(tb.blockKey.value).toBe('')

    await settle()
    expect(store.accountUid).toBe(7)
    expect(tb.blockKey.value).toBe('exclude_tag_trees=6&exclude_tag_ids=9')
  })

  it("a logout forgets the account's list but keeps the browser's", async () => {
    const store = useTagBlockStore()
    store.anonymous = { tree: [1], exact: [] }
    signIn()
    await settle()
    expect(store.accountUid).toBe(7)

    useUserStore().logout()
    await settle()
    expect(store.accountUid).toBe(0)
    expect(store.account).toEqual({ tree: [], exact: [] })
    expect(useTagBlocks().blockKey.value).toBe('exclude_tag_trees=1')
  })

  it('stops at the cap but still lets a blocked tag change mode', async () => {
    const full = Array.from({ length: 100 }, (_, i) => i + 1)
    useTagBlockStore().anonymous = { tree: full, exact: [] }
    const tb = useTagBlocks()

    expect(await tb.block(101)).toBe(false)
    expect(messages).toEqual(['最多屏蔽 100 个标签'])
    expect(await tb.block(50, 'exact')).toBe(true)
    expect(tb.count.value).toBe(100)
    expect(tb.modeOf(50)).toBe('exact')
  })
})
