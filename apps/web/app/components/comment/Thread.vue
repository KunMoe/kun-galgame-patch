<script setup lang="ts">
// A root comment + its replies, rendered as ONE visual tier (replies sit in
// the root's content column via Item's #replies slot — one indent + a smaller
// avatar, matching kungal /galgame/:id). Inline shows the first few replies; the
// rest expand IN PLACE below via a "展开更多" toggle (`expanded` is controlled by
// the container so a deep-link jump can open the right thread).
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
  toggleExpand: [rootId: number]
}>()

const INLINE_LIMIT = 3
const replies = computed(() => props.root.reply ?? [])
const visibleReplies = computed(() =>
  props.expanded ? replies.value : replies.value.slice(0, INLINE_LIMIT)
)
// More replies than the inline preview → show the expand / collapse toggle.
const canToggle = computed(() => replies.value.length > INLINE_LIMIT)
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
          v-if="canToggle"
          variant="light"
          color="primary"
          size="sm"
          full-width
          class-name="mt-3"
          @click="emit('toggleExpand', root.id)"
        >
          <KunIcon
            :name="expanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
            class="size-4"
          />
          {{
            expanded
              ? '收起回复'
              : `展开更多 ${replies.length - INLINE_LIMIT} 条回复`
          }}
        </KunButton>
      </template>
    </CommentItem>
  </KunCard>
</template>
