import { describe, expect, it } from 'vitest'

const state = (unread: number, requests: number): ChatState => ({
  last_update_seq: 1,
  unread_conversation_count: unread,
  unread_message_count: unread,
  request_count: requests
})

describe('chatNoticeOf', () => {
  it('stays dark before chat has started or when it is down', () => {
    expect(chatNoticeOf('idle', null, false)).toBe('none')
    expect(chatNoticeOf('down', null, false)).toBe('none')
    expect(chatNoticeOf('ready', state(0, 0), false)).toBe('none')
  })

  it('lights for a session that lacks the chat scope', () => {
    expect(chatNoticeOf('scope', null, false)).toBe('scope')
    expect(chatNoticeOf('scope', state(3, 1), false)).toBe('scope')
  })

  it('prefers unread conversations over requests', () => {
    expect(chatNoticeOf('ready', state(2, 1), false)).toBe('unread')
    expect(chatNoticeOf('ready', state(0, 1), false)).toBe('requests')
  })

  it('stays dark for every case once direct messages are muted', () => {
    expect(chatNoticeOf('scope', null, true)).toBe('none')
    expect(chatNoticeOf('ready', state(2, 1), true)).toBe('none')
    expect(chatNoticeOf('ready', state(0, 1), true)).toBe('none')
  })
})

describe('chatNoticeHref', () => {
  it('lands on the requests folder only when requests are all there is', () => {
    expect(chatNoticeHref('requests')).toBe('/messages?folder=requests')
    expect(chatNoticeHref('unread')).toBe('/messages')
    expect(chatNoticeHref('scope')).toBe('/messages')
    expect(chatNoticeHref('none')).toBe('/messages')
  })
})

describe('isChatFolder', () => {
  it('accepts only the three folders', () => {
    expect(isChatFolder('requests')).toBe(true)
    expect(isChatFolder('inbox')).toBe(true)
    expect(isChatFolder('archive')).toBe(true)
    expect(isChatFolder('all')).toBe(false)
    expect(isChatFolder(undefined)).toBe(false)
    expect(isChatFolder(['requests'])).toBe(false)
  })
})
