<script setup lang="ts">
import type { BrowseTag } from '~/composables/useTaxonomyBrowse'

const props = defineProps<{ tag: BrowseTag }>()

const { blockOf } = useTagBlocks()

const notice = computed(() => {
  const { id, name, ancestors, descendant_count } = props.tag
  const block = blockOf(
    id,
    ancestors.map((a) => a.id)
  )
  if (!block) return null
  if (block.tagId === id) {
    const scope = block.mode === 'tree' && descendant_count ? '或其子标签' : ''
    return {
      title: '该标签已被屏蔽',
      text: `带有「${name}」${scope}的 Galgame 不会出现在全站列表中，所以这里也不会显示。`
    }
  }
  const ancestor = ancestors.find((a) => a.id === block.tagId)
  if (!ancestor) return null
  return {
    title: '上级标签已被屏蔽',
    text: `您屏蔽了「${ancestor.name}」及其子标签，「${name}」也在其中，带有它的 Galgame 不会出现在全站列表中，所以这里也不会显示。`
  }
})
</script>

<template>
  <KunInfo v-if="notice" color="warning" variant="flat" :title="notice.title">
    <p>{{ notice.text }}</p>
    <KunButton
      size="xs"
      variant="flat"
      color="warning"
      class-name="mt-2"
      @click="navigateTo('/settings/system#tag-blocks')"
    >
      <KunIcon name="lucide:ban" class="size-3.5" />
      管理屏蔽标签
    </KunButton>
  </KunInfo>
</template>
