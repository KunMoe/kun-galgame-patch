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
import { linkSchema } from '@milkdown/kit/preset/commonmark'
import { TextSelection } from '@milkdown/kit/prose/state'
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
    shouldShow(targetView) {
      const content = this.getContent(targetView)
      if (!content) return false
      const lastAt = content.lastIndexOf('@')
      if (lastAt < 0) return false
      const after = content.slice(lastAt + 1)
      // Any whitespace after the @ ends the mention run. /\s/ also catches
      // full-width / ideographic spaces that endsWith(' ') would miss.
      if (/\s/.test(after)) return false
      query.value = after
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
    const { from, $from } = state.selection
    const textContent = $from.node().textContent
    const untilAt = textContent.lastIndexOf('@')
    if (untilAt < 0) return
    const offset = textContent.length - untilAt
    const link = linkSchema
      .type(ctx)
      .create({ href: `/user/${user.id}/resource` })
    // Only the "@name" text carries the link mark; the trailing space MUST be
    // unmarked. Otherwise the caret sits inside the link and whatever the user
    // types next (e.g. a URL) is pulled INTO the mention's link (the "粘连" bug).
    const mention = state.schema.text(`@${user.name}`).mark([link])
    const space = state.schema.text(' ')
    if (from - offset >= 0) {
      const tr = state.tr.replaceWith(from - offset, from, [mention, space])
      // Caret after the unmarked space + stored marks cleared, so typing keeps
      // going as plain text rather than extending the mention link.
      const after = from - offset + mention.nodeSize + space.nodeSize
      tr.setSelection(TextSelection.create(tr.doc, after))
      tr.setStoredMarks([])
      dispatch(tr)
      targetView.focus()
    }
  })
  users.value = []
  query.value = ''
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
