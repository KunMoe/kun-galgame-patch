<script setup lang="ts">
// A root comment + its replies, rendered as ONE visual tier (replies sit in
// the root's content column via Item's #replies slot — one indent + a smaller
// avatar, matching kungal /galgame/:id). Inline shows the first few replies;
// the rest open in the ThreadDrawer (`expanded` renders them all, no button).
//
// Pure presentation: every mutation bubbles up to the container (comment.vue),
// which owns the comments array.
const props = withDefaults(
  defineProps<{
    root: PatchPageComment
    galgameId: number
    expanded?: boolean
    canModerate?: boolean
  }>(),
  { expanded: false, canModerate: false }
)

const emit = defineEmits<{
  liked: [id: number, liked: boolean]
  replyAdded: [reply: PatchPageComment]
  edited: [updated: PatchPageComment]
  removed: [id: number]
  openThread: [rootId: number]
}>()

const INLINE_LIMIT = 3
const replies = computed(() => props.root.reply ?? [])
const visibleReplies = computed(() =>
  props.expanded ? replies.value : replies.value.slice(0, INLINE_LIMIT)
)
const hasMore = computed(
  () => !props.expanded && replies.value.length > INLINE_LIMIT
)
</script>

<template>
  <KunCard :bordered="true" :is-hoverable="false">
    <CommentItem
      :comment="root"
      :galgame-id="galgameId"
      :root-id="root.id"
      :depth="0"
      :can-moderate="canModerate"
      @liked="(id, liked) => emit('liked', id, liked)"
      @reply-added="(r) => emit('replyAdded', r)"
      @edited="(u) => emit('edited', u)"
      @removed="(id) => emit('removed', id)"
    >
      <template #replies>
        <div v-if="visibleReplies.length" class="mt-3 space-y-4">
          <CommentItem
            v-for="r in visibleReplies"
            :key="r.id"
            :comment="r"
            :galgame-id="galgameId"
            :root-id="root.id"
            :depth="1"
            :can-moderate="canModerate"
            @liked="(id, liked) => emit('liked', id, liked)"
            @reply-added="(rr) => emit('replyAdded', rr)"
            @edited="(u) => emit('edited', u)"
            @removed="(id) => emit('removed', id)"
          />
        </div>

        <KunButton
          v-if="hasMore"
          variant="light"
          color="primary"
          size="sm"
          full-width
          class-name="mt-3"
          @click="emit('openThread', root.id)"
        >
          <KunIcon name="lucide:messages-square" class="size-4" />
          查看全部 {{ replies.length }} 条回复
        </KunButton>
      </template>
    </CommentItem>
  </KunCard>
</template>
