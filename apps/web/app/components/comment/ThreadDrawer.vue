<script setup lang="ts">
// Side drawer showing a full comment thread (root + ALL replies). moyu's
// GetComments already returns every reply inline, so the drawer just renders
// the same reactive root object with `expanded` — no extra fetch. The root is
// shared with the container's array, so in-drawer mutations (reply/edit/delete)
// reflected by the container show here live, and vice-versa.
//
// `root` is the v-model: a non-null comment opens the drawer; closing clears it.
const root = defineModel<PatchPageComment | null>('root', { default: null })

defineProps<{
  galgameId: number
  canModerate?: boolean
}>()

const emit = defineEmits<{
  liked: [id: number, liked: boolean]
  replyAdded: [reply: PatchPageComment]
  edited: [updated: PatchPageComment]
  removed: [id: number]
}>()

const isOpen = computed({
  get: () => root.value !== null,
  set: (value) => {
    if (!value) root.value = null
  }
})
</script>

<template>
  <KunDrawer v-model="isOpen" title="回复详情" size="lg">
    <CommentThread
      v-if="root"
      :root="root"
      :galgame-id="galgameId"
      :expanded="true"
      :can-moderate="canModerate"
      @liked="(id, liked) => emit('liked', id, liked)"
      @reply-added="(r) => emit('replyAdded', r)"
      @edited="(u) => emit('edited', u)"
      @removed="(id) => emit('removed', id)"
    />
  </KunDrawer>
</template>
