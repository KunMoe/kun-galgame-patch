import { describe, expect, it } from 'vitest'
import { nextmoeEditorWorkUrl } from './nextmoeEditor'

describe('nextmoeEditorWorkUrl', () => {
  it('deep-links the work editor with the return page and account hint', () => {
    const url = new URL(
      nextmoeEditorWorkUrl({
        base: 'https://editor.nextmoe.com',
        workId: 4,
        returnUrl: 'https://www.moyu.moe/galgame/4?tab=resource',
        loginHint: '0f5c2c5e-1f1e-4d7a-9a51-6a0a3c2e9b10'
      })
    )
    expect(url.origin + url.pathname).toBe(
      'https://editor.nextmoe.com/works/4/edit'
    )
    expect(url.searchParams.get('from')).toBe('moyu')
    expect(url.searchParams.get('return')).toBe(
      'https://www.moyu.moe/galgame/4?tab=resource'
    )
    expect(url.searchParams.get('login_hint')).toBe(
      '0f5c2c5e-1f1e-4d7a-9a51-6a0a3c2e9b10'
    )
  })

  it('leaves login_hint out for a signed-out reader', () => {
    const url = new URL(
      nextmoeEditorWorkUrl({
        base: 'https://editor.nextmoe.com/',
        workId: 12,
        returnUrl: 'https://www.moyu.moe/galgame/12'
      })
    )
    expect(url.searchParams.has('login_hint')).toBe(false)
  })
})
