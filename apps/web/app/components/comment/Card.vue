<script setup lang="ts">
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

const target = computed(
  () => `/patch/${props.comment.galgame_id}/comment#comment-${props.comment.id}`
)

// The card MUST NOT render as a NuxtLink (i.e. don't pass `:href`). Rendered
// comment Markdown carries its own <a> (@mentions, autolinks, KunAvatar's link),
// and nesting <a> inside <a> triggers a Vue hydration mismatch. Keep as a plain
// div + imperative navigation; defer to any inner <a>/<button> the user clicked.
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
        <!-- KunContent: sanitize + spoiler + inline-image lightbox built in.
             Its lightbox click stops propagation, so tapping an image opens
             the viewer without triggering this card's handleCardClick. -->
        <KunContent :content="props.comment.content_html || ''" class-name="mt-1" />
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
