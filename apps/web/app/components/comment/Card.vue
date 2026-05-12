<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

interface Props {
  comment: PatchComment
}

const props = defineProps<Props>()

// The home / global-comments endpoints serialize `User *PatchUser` with
// `omitempty`, so a comment whose author isn't returned by OAuth /users/batch
// (deleted, banned, not_found) arrives with `user === undefined`. Without
// guarding, `props.comment.user.name` throws during render — SSR can still
// emit the avatar DOM that ran before the throw, while CSR hydration aborts
// the whole subtree to a comment vnode, which is exactly the
// "rendered on server: <div>… / expected on client: Symbol(v-cmt)" symptom
// reported on the homepage comments.
const safeUser = computed(() => props.comment.user ?? null)
const displayName = computed(() => safeUser.value?.name ?? '已注销用户')

const patchName = computed(() =>
  props.comment.patch?.name
    ? getPreferredLanguageText(props.comment.patch.name)
    : `补丁 #${props.comment.galgame_id}`
)

const contentHtml = computed(() =>
  DOMPurify.sanitize(props.comment.content_html || '', {
    ADD_ATTR: ['data-uid']
  })
)

const target = computed(
  () => `/patch/${props.comment.galgame_id}/comment`
)

// The card was previously rendered as a NuxtLink (KunCard isPressable). That
// nested every <a> inside the comment's rendered Markdown — @mentions, auto-
// links, even KunAvatar's internal link — under an outer <a>, which Vue flags
// as a hydration mismatch. We now keep the card as a plain div and navigate
// imperatively, deferring to any inner <a>/<button> the user actually clicked.
const handleCardClick = async (event: MouseEvent) => {
  const el = event.target as HTMLElement | null
  if (el?.closest('a, button')) return
  await navigateTo(target.value)
}

const handleKeydown = async (event: KeyboardEvent) => {
  if (event.key !== 'Enter' && event.key !== ' ') return
  const el = event.target as HTMLElement | null
  if (el?.closest('a, button')) return
  event.preventDefault()
  await navigateTo(target.value)
}
</script>

<template>
  <KunCard
    is-hoverable
    class-name="w-full cursor-pointer"
    role="link"
    :tabindex="0"
    :aria-label="`查看 ${displayName} 的评论`"
    @click="handleCardClick"
    @keydown="handleKeydown"
  >
    <div class="flex gap-4">
      <KunAvatar :user="safeUser" />
      <div class="space-y-2">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="font-semibold">{{ displayName }}</h2>
          <span class="text-small text-default-500">
            评论在
            <span class="text-primary-500">{{ patchName }}</span>
          </span>
        </div>
        <div class="kun-prose mt-1" v-html="contentHtml" />
        <div class="mt-2 flex items-center gap-4">
          <div class="text-small text-default-500 flex items-center gap-1">
            <KunIcon name="lucide:thumbs-up" class="size-3.5" />
            {{ props.comment.like_count }}
          </div>
          <span class="text-small text-default-500">
            {{
              formatDate(props.comment.created, {
                isPrecise: true,
                isShowYear: true
              })
            }}
          </span>
        </div>
      </div>
    </div>
  </KunCard>
</template>
