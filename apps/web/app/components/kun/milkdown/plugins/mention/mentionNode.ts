import type { MilkdownPlugin } from '@milkdown/kit/ctx'
import type { Node } from '@milkdown/kit/transformer'
import { $nodeSchema, $remark } from '@milkdown/kit/utils'
import { visit } from 'unist-util-visit'
import type { Node as UnistNode } from 'unist'

// @mention as an inline ATOM node (not a text link).
//
// Stored markdown form is unchanged: `[@name](/user/<id>/resource)` — the same
// link the backend's mentionPatternRegex (`\[@[^\]]*\]\(/user/(\d+)...`) parses
// for notifications, and that goldmark renders as a link for display. Only the
// EDITOR representation changes: a `$remark` transformer rewrites those links
// into a `mention` atom node on PARSE, and toMarkdown turns the node back into
// the same link on serialize. (remark transformers run on parse only, so the
// emitted link is left alone — same round-trip trick as the spoiler plugin.)
//
// Why an atom node instead of a link mark: once completed, the mention is an
// opaque leaf, so (1) the @-search detector never re-matches its "@name" text
// (it reads as a single leaf char, not "@name"), and (2) there is no
// inclusive:false link-mark boundary at the caret, which is what corrupted IME
// composition (first CJK letter committing as Latin). See the mention research.

export const mentionId = 'mention'

// Matches the mention link target: /user/<id> optionally followed by /resource.
const MENTION_URL = /^\/user\/(\d+)(?:\/|$)/

interface MdastNode extends Node {
  type: string
  url?: string
  value?: string
  userId?: number
  name?: string
  children?: MdastNode[]
}

export const mentionSchema = $nodeSchema(mentionId, () => ({
  group: 'inline',
  inline: true,
  atom: true,
  attrs: {
    userId: { default: 0 },
    name: { default: '' }
  },
  parseDOM: [
    {
      // Copy/paste of a chip from within the editor.
      tag: 'span.kun-mention',
      getAttrs: (dom) => {
        const el = dom as HTMLElement
        return {
          userId: Number.parseInt(el.dataset.uid ?? '0', 10) || 0,
          name: (el.textContent ?? '').replace(/^@/, '')
        }
      }
    }
  ],
  toDOM: (node) => {
    // A non-navigating chip inside the editor (a real <a> would steal the click
    // and leave the page mid-compose); contenteditable=false makes it a single
    // opaque caret stop. data-uid mirrors the stored id.
    const span = document.createElement('span')
    span.className = 'kun-mention'
    span.dataset.uid = String(node.attrs.userId)
    span.setAttribute('contenteditable', 'false')
    span.textContent = `@${node.attrs.name}`
    return span
  },
  parseMarkdown: {
    match: (node) => node.type === mentionId,
    runner: (state, node, type) => {
      const n = node as MdastNode
      state.addNode(type, { userId: n.userId ?? 0, name: n.name ?? '' })
    }
  },
  toMarkdown: {
    match: (node) => node.type.name === mentionId,
    runner: (state, node) => {
      state.openNode('link', undefined, {
        url: `/user/${node.attrs.userId}/resource`
      })
      state.addNode('text', undefined, `@${node.attrs.name}`)
      state.closeNode()
    }
  }
}))

// On parse, rewrite mention links (`[@name](/user/<id>/resource)`) into mention
// nodes so the schema only ever matches its own type. A link counts as a mention
// only when its target is /user/<id> AND its text starts with '@' — so ordinary
// links to a user page (non-@ text) stay plain links.
export const remarkMentionPlugin = $remark('remarkMention', () => () => {
  const transformer = (tree: UnistNode) => {
    visit(tree, 'link', (node: MdastNode, index, parent?: MdastNode) => {
      if (typeof node.url !== 'string' || !parent || typeof index !== 'number') {
        return
      }
      const m = node.url.match(MENTION_URL)
      if (!m) return
      const first = node.children?.[0]
      const text = typeof first?.value === 'string' ? first.value : ''
      if (!text.startsWith('@')) return
      const userId = Number.parseInt(m[1] ?? '', 10)
      if (!Number.isInteger(userId) || userId <= 0) return
      parent.children?.splice(index, 1, {
        type: mentionId,
        userId,
        name: text.replace(/^@/, '')
      } as MdastNode)
    })
  }
  return transformer
})

export const kunMentionNodePlugin: MilkdownPlugin[] = [
  mentionSchema,
  remarkMentionPlugin
].flat()
