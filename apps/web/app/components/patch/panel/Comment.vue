<script setup lang="ts">
import type { CommentTarget } from '~/shared/utils/commentTarget'

const route = useRoute()
const userStore = useUserStore()

const galgameId = computed(() => Number(route.params.id))
const target = computed<CommentTarget>(() => ({
  kind: 'patch',
  galgameId: galgameId.value
}))

const {
  items,
  totalPages,
  pending,
  page,
  expandedRoots,
  toggleExpand,
  onLiked,
  onCommentAdded,
  onReplyAdded,
  onEdited,
  onRemoved
} = useCommentList(target, { routeQueryKey: 'page' })
</script>

<template>
  <CommentSection
    v-model:page="page"
    :target="target"
    :items="items"
    :total-pages="totalPages"
    :expanded-roots="expandedRoots"
    :pending="pending"
    :can-moderate="userStore.isModerator"
    @comment-added="onCommentAdded"
    @liked="onLiked"
    @reply-added="onReplyAdded"
    @edited="onEdited"
    @removed="onRemoved"
    @toggle-expand="toggleExpand"
  />
</template>
