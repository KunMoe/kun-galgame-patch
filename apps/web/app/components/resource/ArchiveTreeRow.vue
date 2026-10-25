<script setup lang="ts">
import type { ArchiveRow } from '~/composables/useArchiveTree'

const props = defineProps<{
  row: ArchiveRow
  index: number
  active: boolean
  entering: boolean
  query: string
  openable: boolean
}>()

const emit = defineEmits<{ activate: [] }>()

interface Segment {
  text: string
  tone?: 'hit' | 'dim'
}

const node = computed(() => (props.row.type === 'node' ? props.row.node : null))

const segments = computed((): Segment[] => {
  const n = node.value
  if (!n) return []
  const at = props.query ? n.name.toLowerCase().indexOf(props.query) : -1
  if (at >= 0) {
    const end = at + props.query.length
    return [
      { text: n.name.slice(0, at) },
      { text: n.name.slice(at, end), tone: 'hit' },
      { text: n.name.slice(end) }
    ]
  }
  if (n.dir) return [{ text: n.name }]
  const [base, ext] = splitExtension(n.name)
  return [{ text: base }, { text: ext, tone: 'dim' }]
})

const icon = computed(() => {
  const n = node.value
  if (props.row.type === 'loading') {
    return {
      name: 'lucide:loader-circle',
      tone: 'text-default-400 animate-spin'
    }
  }
  if (!n) return { name: 'lucide:ellipsis', tone: 'text-default-400' }
  if (n.dir) {
    return {
      name:
        props.row.type === 'node' && props.row.open
          ? 'lucide:folder-open'
          : 'lucide:folder',
      tone: 'text-primary'
    }
  }
  const kind = fileKindOf(n.name)
  return { name: kind.icon, tone: kind.tone }
})

const kindLabel = computed(() => {
  const n = node.value
  if (!n) return ''
  return n.dir ? `${n.items} 项` : fileKindOf(n.name).label
})
</script>

<template>
  <div
    role="treeitem"
    :aria-level="row.depth + 1"
    :aria-expanded="node?.dir ? row.type === 'node' && row.open : undefined"
    :tabindex="active ? 0 : -1"
    :data-index="index"
    :title="openable ? `${node?.path}\n点击查看` : node?.path"
    :style="{ '--depth': row.depth }"
    :class="
      cn(
        'group relative grid h-9 cursor-default grid-cols-[minmax(0,1fr)_auto] items-center gap-3 pr-3 text-sm outline-none select-none sm:h-8 sm:grid-cols-[minmax(0,1fr)_7rem_5rem]',
        openable && 'cursor-pointer',
        'pl-[calc(var(--depth)*0.875rem+0.5rem)] sm:pl-[calc(var(--depth)*1rem+0.5rem)]',
        'hover:bg-default-100 focus-visible:bg-primary-50 focus-visible:ring-primary-300 focus-visible:ring-1 focus-visible:ring-inset',
        entering && 'row-in'
      )
    "
    @click="emit('activate')"
  >
    <span
      v-for="guide in row.depth"
      :key="guide"
      aria-hidden="true"
      class="bg-default-200 pointer-events-none absolute inset-y-0 left-[calc(var(--guide)*0.875rem+0.9375rem)] w-px sm:left-[calc(var(--guide)*1rem+0.9375rem)]"
      :style="{ '--guide': guide - 1 }"
    />

    <div class="flex min-w-0 items-center gap-1.5">
      <KunIcon
        v-if="node?.dir"
        name="lucide:chevron-right"
        :class="
          cn(
            'text-default-400 size-3.5 shrink-0 transition-transform duration-150 motion-reduce:transition-none',
            row.type === 'node' && row.open && 'rotate-90'
          )
        "
      />
      <span v-else class="size-3.5 shrink-0" />
      <KunIcon :name="icon.name" :class="cn('size-4 shrink-0', icon.tone)" />
      <span v-if="row.type === 'more'" class="text-primary text-xs">
        显示其余 {{ row.rest.toLocaleString('zh-CN') }} 项
      </span>
      <span v-else-if="row.type === 'loading'" class="text-default-400 text-xs">
        正在加载
      </span>
      <span
        v-else
        :class="
          cn('truncate', openable && 'text-primary group-hover:underline')
        "
      >
        <span
          v-for="(s, i) in segments"
          :key="i"
          :class="
            s.tone === 'hit'
              ? 'text-primary font-medium'
              : s.tone === 'dim'
                ? 'text-default-400'
                : ''
          "
          >{{ s.text }}</span
        >
      </span>
      <KunIcon
        v-if="openable"
        name="lucide:eye"
        class="text-primary size-3.5 shrink-0"
      />
    </div>

    <template v-if="node">
      <span class="text-default-400 hidden truncate text-xs sm:block">
        {{ kindLabel }}
      </span>
      <span class="text-default-500 text-right text-xs tabular-nums">
        {{ formatBytes(node.size) }}
      </span>
    </template>
  </div>
</template>

<style scoped>
.row-in {
  animation: row-in 160ms ease-out both;
}

@keyframes row-in {
  from {
    opacity: 0;
    transform: translateY(-3px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .row-in {
    animation: none;
  }
}
</style>
