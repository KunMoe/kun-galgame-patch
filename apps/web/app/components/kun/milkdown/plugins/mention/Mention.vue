<script setup lang="ts">
// @-mention autocomplete for the Milkdown editor.
//
// Ported from refs/legacy/next-web's MentionsListDropdown: a SlashProvider
// watches the current text block for an unbroken "@query" run, we debounce a
// GET /user/search?query=, and inserting a pick writes a link node
// `[@name](/user/:id/resource)`. That exact shape is what the backend's
// mentionPatternRegex (`\[@[^\]]*\]\(/user/(\d+)...`) parses to fire mention
// notifications, so the FE insert and BE extraction stay in lockstep.
import { SlashProvider } from '@milkdown/kit/plugin/slash'
import { editorViewCtx } from '@milkdown/kit/core'
import { TextSelection } from '@milkdown/kit/prose/state'
import { mentionSchema } from './mentionNode'
import { useInstance } from '@milkdown/vue'
import { usePluginViewContext } from '@prosemirror-adapter/vue'
import { getRandomSticker } from '@kungal/ui-core'
import { resolveAvatarUrl } from '~/shared/utils/resolveAvatarUrl'
import { getMentionApiBase } from './config'

interface MentionUser {
  id: number
  name: string
  avatar?: string
  // Hash-addressed avatar (image_service). Preferred over the legacy avatar
  // URL — resolveAvatarUrl handles both, and the right `_100` variant. We
  // resolve here + render a plain <img> rather than KunAvatar, because
  // KunAvatar only uses .avatar and mangles small sizes to a `-100.webp` (dash)
  // variant that the image_service (underscore `_100`) doesn't serve.
  avatar_image_hash?: string
}

const avatarSrc = (u: MentionUser) =>
  resolveAvatarUrl(u, '100') || getRandomSticker(u.name)

const { view, prevState } = usePluginViewContext()
const [, get] = useInstance()

const divRef = ref<HTMLElement>()
let slashProvider: SlashProvider | undefined

const users = ref<MentionUser[]>([])
const query = ref('')
const fetching = ref(false)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
watch(query, (q) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  const trimmed = q.trim()
  // The backend requires query length >= 1; whitespace already hides the
  // dropdown (shouldShow), but guard here too so we never fire a bad request.
  if (!trimmed || /\s/.test(q)) {
    users.value = []
    return
  }
  debounceTimer = setTimeout(() => fetchUsers(trimmed), 300)
})

const fetchUsers = async (q: string) => {
  fetching.value = true
  try {
    const base = getMentionApiBase()
    const res = await $fetch<{ code: number; data: MentionUser[] }>(
      `${base}/user/search?query=${encodeURIComponent(q)}`,
      { credentials: 'include' }
    ).catch(() => null)
    users.value = res && res.code === 0 ? (res.data ?? []) : []
  } finally {
    fetching.value = false
  }
}

// SlashProvider positions the dropdown from the cursor's viewport coords once
// per update. Page/ancestor scroll moves the cursor but doesn't re-trigger an
// update, so the dropdown drifts out of place — re-run update() on scroll/resize
// to keep it pinned to the caret. `true` (capture) catches scrollable ancestors.
const reposition = () => slashProvider?.update(view.value, prevState.value)

onMounted(() => {
  const el = divRef.value
  if (!el) return
  slashProvider = new SlashProvider({
    content: el,
    // 0 = react immediately. The default 200ms debounce left the dropdown
    // lingering after a pick and while typing the next characters (it only hid
    // after a 200ms pause), which read as "still searching". kungal uses 0 too.
    debounce: 0,
    shouldShow(targetView) {
      // Never show / re-search mid IME composition: touching reactive state
      // here (query → fetch → dropdown re-render) as composition starts can
      // abort it, committing the first CJK letter as Latin. SlashProvider's own
      // #onUpdate guards composing, but we set `query` here, so guard too.
      if (targetView.composing) return false
      const { selection } = targetView.state
      // Only a caret (empty selection) in a text block can carry an @query.
      if (!selection.empty) return false
      const { $from } = selection
      // Text of the current block from its start up to the caret, read straight
      // from the doc — NOT SlashProvider.getContent, whose default doesn't
      // reliably include a trailing space, so a just-inserted mention's unmarked
      // trailing space failed to end the run and typing kept re-searching.
      const before = $from.parent.textBetween(
        0,
        $from.parentOffset,
        undefined,
        '￼'
      )
      // @query anchored at the caret: '@' must start the block or follow
      // whitespace (so emails like foo@bar never trigger), then up to 30
      // non-space / non-@ chars ending exactly at the caret. A completed mention
      // ends with "@name " — the trailing space breaks the match, so continuing
      // to type no longer searches. `\s` also covers the full-width space.
      const match = before.match(/(?:^|\s)@([^\s@]{0,30})$/)
      if (!match) return false
      query.value = match[1] ?? ''
      return true
    }
  })
  slashProvider.update(view.value, prevState.value)
  window.addEventListener('scroll', reposition, true)
  window.addEventListener('resize', reposition)
})

watch([view, prevState], () => {
  slashProvider?.update(view.value, prevState.value)
})

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  window.removeEventListener('scroll', reposition, true)
  window.removeEventListener('resize', reposition)
  slashProvider?.destroy()
})

const pickUser = (user: MentionUser) => {
  if (!user.name) return
  get()?.action((ctx) => {
    const targetView = ctx.get(editorViewCtx)
    const { dispatch, state } = targetView
    const { $from } = state.selection
    // Find the "@query" run ending at the caret (same detection as shouldShow)
    // so we replace exactly it — never a leading space or earlier text.
    const before = $from.parent.textBetween(
      Math.max(0, $from.parentOffset - 100),
      $from.parentOffset,
      undefined,
      '￼'
    )
    const match = before.match(/(?:^|\s)@([^\s@]{0,30})$/)
    if (!match) return
    const queryLen = (match[1]?.length ?? 0) + 1 // '@' + query text
    const from = $from.pos - queryLen
    const to = $from.pos
    if (from < 0) return
    // Insert the mention as an opaque ATOM node + a trailing space, caret after
    // the space. The atom can't be re-matched as "@query" text and has no link
    // mark, so typing afterwards (incl. IME) never re-searches or bleeds.
    const node = mentionSchema.type(ctx).create({
      userId: user.id,
      name: user.name
    })
    const tr = state.tr.replaceWith(from, to, node)
    const after = from + node.nodeSize
    tr.insertText(' ', after)
    tr.setSelection(TextSelection.create(tr.doc, after + 1))
    dispatch(tr.scrollIntoView())
    targetView.focus()
  })
  users.value = []
  query.value = ''
  // Close the dropdown right now instead of waiting for the next provider
  // update — otherwise it stays visible over the just-inserted mention.
  slashProvider?.hide()
}
</script>

<template>
  <!-- SlashProvider positions this element at the cursor by setting inline
       top/left (needs `absolute`) and toggles a data-show attribute
       (data-[show='false']:hidden). It must always be in the DOM. The anchor is
       ProseMirror's editable element, which is position:relative by default.
       mousedown.prevent keeps editor focus so the insert's selection math stays
       valid (a click would blur first). -->
  <div
    ref="divRef"
    class="kun-mention-dropdown border-default-200 bg-content1 z-kun-popover absolute data-[show='false']:hidden max-h-64 w-64 overflow-y-auto rounded-lg border p-1 shadow-lg"
  >
    <ul v-if="users.length" class="space-y-0.5">
      <li
        v-for="u in users"
        :key="u.id"
        class="hover:bg-default-100 flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5"
        @mousedown.prevent="pickUser(u)"
      >
        <img
          :src="avatarSrc(u)"
          :alt="u.name"
          class="size-6 shrink-0 rounded-full object-cover"
        />
        <span class="min-w-0 flex-1 truncate text-sm">{{ u.name }}</span>
      </li>
    </ul>
    <div v-else-if="fetching" class="text-default-500 px-2 py-1.5 text-sm">
      搜索中...
    </div>
    <div v-else class="text-default-400 px-2 py-1.5 text-sm">
      继续输入以查找用户
    </div>
  </div>
</template>
