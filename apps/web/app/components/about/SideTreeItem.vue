<script setup lang="ts">
// Recursive tree item used by AboutSidebar. Directories collapse on click;
// files navigate via NuxtLink. Active highlighting is keyed by `path`, which
// matches the slug under apps/web/posts.
interface Props {
  node: KunTreeNode
  level: number
  activeSlug?: string
}

const props = withDefaults(defineProps<Props>(), { level: 0, activeSlug: '' })

// Default collapsed state: open the directory containing the active slug, or
// open at the top level when no slug is selected.
const isOpen = ref(
  props.node.type === 'directory' &&
    (props.level === 0 ||
      (!!props.activeSlug && props.activeSlug.startsWith(props.node.path + '/')))
)

const toggle = () => {
  if (props.node.type === 'directory') isOpen.value = !isOpen.value
}

const isActive = computed(
  () => props.node.type === 'file' && props.node.path === props.activeSlug
)

const indentPx = computed(() => `${props.level * 12 + 12}px`)

const rowClass = computed(() => [
  'hover:bg-default-100 mt-1 flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors',
  isActive.value ? 'bg-primary/10 text-primary font-medium' : 'text-foreground'
])
</script>

<template>
  <div class="select-none">
    <NuxtLink
      v-if="props.node.type === 'file'"
      :to="`/about/${props.node.path}`"
      :class="rowClass"
      :style="{ paddingLeft: indentPx }"
    >
      <KunIcon
        name="lucide:file-text"
        class="text-primary ml-5 size-4 shrink-0"
      />
      <span class="text-sm leading-tight break-words">
        {{ props.node.label || props.node.name }}
      </span>
    </NuxtLink>

    <button
      v-else
      type="button"
      :class="rowClass"
      :style="{ paddingLeft: indentPx }"
      @click="toggle"
    >
      <KunIcon
        name="lucide:chevron-right"
        :class="cn('size-4 transition-transform duration-200', isOpen && 'rotate-90')"
      />
      <KunIcon name="lucide:folder-open" class="text-warning size-4" />
      <span class="text-sm leading-tight break-words">
        {{ props.node.label || props.node.name }}
      </span>
    </button>

    <div
      v-if="props.node.type === 'directory' && isOpen && props.node.children?.length"
    >
      <AboutSideTreeItem
        v-for="(child, i) in props.node.children"
        :key="i"
        :node="child"
        :level="props.level + 1"
        :active-slug="props.activeSlug"
      />
    </div>
  </div>
</template>
