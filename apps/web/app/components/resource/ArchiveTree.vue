<script setup lang="ts">
import type { ArchiveEntry, ArchiveNode } from '~/shared/utils/archiveTree'
import type { ArchiveRow } from '~/composables/useArchiveTree'

const props = defineProps<{
  entries: ArchiveEntry[]
  canOpen?: (node: ArchiveNode) => boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  open: [node: ArchiveNode, siblings: ArchiveNode[]]
  load: []
}>()

const root = computed(() => buildArchiveTree(props.entries))

const openable = (row: ArchiveRow) =>
  row.type === 'node' && !row.node.dir && !!props.canOpen?.(row.node)
const {
  query,
  filter,
  rows,
  isOpen,
  justOpened,
  toggle,
  showMore,
  collapseAll
} = useArchiveTree(root)

watch(
  () => !!query.value.trim() || rows.value.some((r) => r.type === 'loading'),
  (need) => need && emit('load')
)

const treeRef = ref<HTMLElement | null>(null)
const activeKey = ref('')
const activeIndex = computed(() =>
  Math.max(
    0,
    rows.value.findIndex((r) => r.key === activeKey.value)
  )
)

const focusRow = (index: number) => {
  const row = rows.value[index]
  if (!row) return
  activeKey.value = row.key
  nextTick(() =>
    treeRef.value
      ?.querySelector<HTMLElement>(`[data-index="${index}"]`)
      ?.focus()
  )
}

const activate = (row: ArchiveRow) => {
  activeKey.value = row.key
  if (row.type === 'loading') return
  if (row.type === 'more') showMore(row.dir)
  else if (row.node.dir) toggle(row.node)
  else if (openable(row)) {
    const parent = findArchiveNode(root.value, row.node.parent)
    emit('open', row.node, parent?.children ?? [row.node])
  }
}

const parentIndex = (row: ArchiveRow) => {
  const parent = row.type === 'node' ? row.node.parent : row.dir.path
  return rows.value.findIndex((r) => r.type === 'node' && r.key === parent)
}

const onKeydown = (e: KeyboardEvent) => {
  const i = activeIndex.value
  const row = rows.value[i]
  if (!row) return
  const dir = row.type === 'node' && row.node.dir ? row.node : null
  switch (e.key) {
    case 'ArrowDown':
      focusRow(i + 1)
      break
    case 'ArrowUp':
      focusRow(i - 1)
      break
    case 'Home':
      focusRow(0)
      break
    case 'End':
      focusRow(rows.value.length - 1)
      break
    case 'ArrowRight':
      if (dir && !isOpen(dir.path)) toggle(dir)
      else if (dir) focusRow(i + 1)
      break
    case 'ArrowLeft':
      if (dir && isOpen(dir.path)) toggle(dir)
      else focusRow(parentIndex(row))
      break
    case 'Enter':
    case ' ':
      activate(row)
      break
    default:
      return
  }
  e.preventDefault()
}
</script>

<template>
  <div class="border-default-200 rounded-kun-md overflow-hidden border">
    <div class="border-default-200 flex items-center gap-2 border-b p-2">
      <div class="min-w-0 flex-1 sm:max-w-xs">
        <KunInput
          v-model="query"
          size="sm"
          placeholder="筛选文件名"
          is-clearable
        >
          <template #prefix>
            <KunIcon name="lucide:search" class="text-default-400 size-4" />
          </template>
        </KunInput>
      </div>
      <span v-if="filter" class="text-default-400 shrink-0 text-xs">
        {{ filter.hits.toLocaleString('zh-CN') }} 个匹配
      </span>
      <KunButton
        class="ml-auto shrink-0"
        size="sm"
        variant="light"
        color="default"
        is-icon-only
        aria-label="全部折叠"
        @click="collapseAll"
      >
        <KunIcon name="lucide:chevrons-down-up" class="size-4" />
      </KunButton>
    </div>

    <div
      aria-hidden="true"
      class="text-default-400 border-default-200 grid h-8 [scrollbar-gutter:stable] grid-cols-[minmax(0,1fr)_auto] items-center gap-3 overflow-hidden border-b pr-3 pl-2 text-xs sm:grid-cols-[minmax(0,1fr)_7rem_5rem]"
    >
      <span>名称</span>
      <span class="hidden sm:block">类型</span>
      <span class="text-right">大小</span>
    </div>

    <div
      ref="treeRef"
      role="tree"
      aria-label="压缩包内容"
      class="max-h-[min(30rem,65vh)] [scrollbar-gutter:stable] overflow-y-auto overscroll-contain py-1"
      @keydown="onKeydown"
    >
      <ResourceArchiveTreeRow
        v-for="(row, i) in rows"
        :key="row.key"
        :row="row"
        :index="i"
        :active="i === activeIndex"
        :entering="row.type === 'node' && justOpened.has(row.node.parent)"
        :query="filter?.query ?? ''"
        :openable="openable(row)"
        @activate="activate(row)"
      />
      <p
        v-if="filter && !filter.hits && !loading"
        class="text-default-400 px-4 py-8 text-center text-sm"
      >
        没有名称包含「{{ query.trim() }}」的文件
      </p>
    </div>
  </div>
</template>
