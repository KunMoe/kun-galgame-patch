<script setup lang="ts">
interface Props {
  comment: PatchComment
}

const props = defineProps<Props>()

const safeUser = computed(() => props.comment.user ?? null)
const displayName = computed(() => safeUser.value?.name ?? '已注销用户')

const patchName = computed(() =>
  props.comment.patch?.name
    ? getPreferredLanguageText(props.comment.patch.name)
    : `补丁 #${props.comment.galgame_id}`
)

const isResourceComment = computed(() => !!props.comment.resource_id)
// The permalink is the server's: only the post's anchor says which of the two
// walls the comment is on, and the id alone cannot be turned back into one.
const target = computed(() => props.comment.link)

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
    padding="sm"
    role="link"
    :tabindex="0"
    :aria-label="`查看 ${displayName} 的评论`"
    @click="handleCardClick"
    @keydown="handleKeydown"
  >
    <div class="flex gap-4">
      <KunAvatar :user="toKunUser(safeUser)" />
      <div class="min-w-0 flex-1 space-y-2">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="font-semibold">{{ displayName }}</h2>
          <span class="text-small text-default-500">
            {{ isResourceComment ? '评论了' : '评论在' }}
            <span class="text-primary-500">{{ patchName }}</span>
            <template v-if="isResourceComment">的补丁资源</template>
          </span>
        </div>
        <KunContent
          compact
          :content="props.comment.content_html || ''"
          class-name="mt-1"
        />
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
