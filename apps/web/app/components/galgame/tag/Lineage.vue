<script setup lang="ts">
import type { BrowseTag } from '~/composables/useTaxonomyBrowse'

const props = defineProps<{ tag: BrowseTag }>()

const { stance } = useKunNsfwStance()

const trail = computed(() =>
  ancestorTrail(props.tag.parents, props.tag.ancestors).map((node) => ({
    ...node,
    linked: stance.value !== 'hide' || !node.is_sexual
  }))
)
const otherParents = computed(() => props.tag.parents.slice(1))
</script>

<template>
  <div
    v-if="trail.length"
    class="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-sm"
  >
    <span class="text-default-500 mr-1">层级</span>
    <template v-for="node in trail" :key="node.id">
      <KunLink
        v-if="node.linked"
        :to="`/galgame/tag/${node.id}`"
        color="default"
        underline="hover"
      >
        {{ node.name }}
      </KunLink>
      <span v-else class="text-default-500">{{ node.name }}</span>
      <KunIcon name="lucide:chevron-right" class="text-default-300 size-3.5" />
    </template>
    <span class="text-foreground font-medium">{{ tag.name }}</span>

    <template v-if="otherParents.length">
      <span class="text-default-500 mr-1 ml-3">也属于</span>
      <KunLink
        v-for="parent in otherParents"
        :key="parent.id"
        :to="`/galgame/tag/${parent.id}`"
        underline="none"
      >
        <KunChip
          size="xs"
          variant="flat"
          :color="parent.is_sexual ? 'danger' : 'default'"
          class-name="cursor-pointer"
        >
          {{ parent.name }}
        </KunChip>
      </KunLink>
    </template>
  </div>
</template>
