import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

const { replies } = vi.hoisted(() => ({
  replies: {} as Record<string, { code: number; data: unknown }>
}))

mockNuxtImport('useApi', () => () => ({
  get: async (endpoint: string) =>
    replies[endpoint.split('?')[0]!] ?? { code: 40400, data: null }
}))

const ok = (data: unknown) => ({ code: 0, data })

const chatState = (unread: number, requests: number) =>
  ok({
    last_update_seq: 1,
    unread_conversation_count: unread,
    unread_message_count: unread,
    request_count: requests
  })

afterEach(() => {
  for (const k of Object.keys(replies)) Reflect.deleteProperty(replies, k)
  useChatStore().stop()
  useUserStore().setMutedMessageTypes([])
})

describe('chat store notice', () => {
  it('lights for a session without the chat scope and leads to the re-login prompt', async () => {
    replies['/im/state'] = { code: 40313, data: null }
    const chat = useChatStore()
    await chat.start('7')

    expect(chat.status).toBe('scope')
    expect(chat.hasUnread).toBe(true)
    expect(chat.href).toBe('/messages')
  })

  it('sends a reader with only requests to the requests folder', async () => {
    replies['/im/state'] = chatState(0, 2)
    replies['/im/conversations'] = ok({ items: [], users: [], next_cursor: '' })
    replies['/im/reactions'] = ok({ items: [] })
    const chat = useChatStore()
    await chat.start('7')

    expect(chat.notice).toBe('requests')
    expect(chat.href).toBe('/messages?folder=requests')
  })

  it('goes dark when the 私信 toggle (pm) is muted', async () => {
    replies['/im/state'] = { code: 40313, data: null }
    const chat = useChatStore()
    await chat.start('7')

    useUserStore().toggleMutedMessageType('pm')
    expect(chat.hasUnread).toBe(false)

    useUserStore().toggleMutedMessageType('pm')
    expect(chat.hasUnread).toBe(true)
  })

  it('stays dark once chat stops, as on logout', async () => {
    replies['/im/state'] = { code: 40313, data: null }
    const chat = useChatStore()
    await chat.start('7')
    chat.stop()

    expect(chat.status).toBe('idle')
    expect(chat.hasUnread).toBe(false)
  })
})
