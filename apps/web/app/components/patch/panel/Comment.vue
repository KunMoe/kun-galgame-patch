<script setup lang="ts">
import type { CommentTarget } from '~/shared/utils/commentTarget'

const route = useRoute()

const galgameId = computed(() => Number(route.params.id))
const target = computed<CommentTarget>(() => ({
  kind: 'patch',
  galgameId: galgameId.value
}))

const emit = defineEmits<{ 'update:loading': [boolean] }>()

const wall = useCommentList(target)

watch(wall.pending, (value) => emit('update:loading', value), {
  immediate: true
})
</script>

<template>
  <CommentSection :target="target" :wall="wall" />
</template>
