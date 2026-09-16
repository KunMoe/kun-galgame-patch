<script setup lang="ts">
import type { CommentTarget } from '~/shared/utils/commentTarget'

const route = useRoute()
const userStore = useUserStore()

const galgameId = computed(() => Number(route.params.id))
const target = computed<CommentTarget>(() => ({
  kind: 'patch',
  galgameId: galgameId.value
}))

const emit = defineEmits<{ 'update:loading': [boolean] }>()

const {
  groups,
  subscription,
  setLevel,
  hasMore,
  loadMore,
  loadingMore,
  pending,
  expandedRoots,
  toggleExpand,
  onLiked,
  onCommentAdded,
  onEdited,
  onRemoved
} = useCommentList(target)

watch(pending, (value) => emit('update:loading', value), { immediate: true })
</script>

<template>
  <CommentSection
    :target="target"
    :groups="groups"
    :expanded-roots="expandedRoots"
    :subscription="subscription"
    :has-more="hasMore"
    :loading-more="loadingMore"
    :pending="pending"
    :can-moderate="userStore.isModerator"
    @comment-added="onCommentAdded"
    @liked="onLiked"
    @edited="onEdited"
    @removed="onRemoved"
    @toggle-expand="toggleExpand"
    @load-more="loadMore"
    @set-level="setLevel"
  />
</template>
