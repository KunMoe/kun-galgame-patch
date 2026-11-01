<script setup lang="ts">
import { SUPPORTED_RESOURCE_LINK_MAP } from '~/constants/resource'

const props = defineProps<{
  patchId: number
  resources: PatchResource[]
}>()

const emit = defineEmits<{
  close: []
  saved: [resourceIds: number[]]
}>()

const api = useApi()

const pinned = ref<PatchResource[]>(
  props.resources
    .filter((r) => r.display_order != null)
    .sort((a, b) => a.display_order! - b.display_order!)
)
const rest = ref<PatchResource[]>(
  props.resources.filter((r) => r.display_order == null)
)

type Zone = 'pinned' | 'rest'
const drag = ref<{ zone: Zone; index: number } | null>(null)
const insertAt = ref<number | null>(null)

const onDragStart = (zone: Zone, index: number, e: DragEvent) => {
  drag.value = { zone, index }
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
  }
}

const ROW =
  'border-default/20 bg-content1 relative flex cursor-grab items-center gap-2 rounded-lg border p-2 active:cursor-grabbing'
const LINE_ABOVE =
  "before:bg-primary before:absolute before:inset-x-0 before:-top-1 before:h-0.5 before:content-['']"
const LINE_BELOW =
  "after:bg-primary after:absolute after:inset-x-0 after:-bottom-1 after:h-0.5 after:content-['']"

const isDragging = (zone: Zone, index: number) =>
  drag.value?.zone === zone && drag.value.index === index

const pinnedRowClass = (i: number) =>
  cn(
    ROW,
    isDragging('pinned', i) && 'opacity-50',
    insertAt.value === i && LINE_ABOVE,
    insertAt.value === i + 1 && i === pinned.value.length - 1 && LINE_BELOW
  )

const onDragEnd = () => {
  drag.value = null
  insertAt.value = null
}

const onPinnedRowOver = (index: number, e: DragEvent) => {
  if (!drag.value) return
  const box = (e.currentTarget as HTMLElement).getBoundingClientRect()
  insertAt.value = e.clientY < box.top + box.height / 2 ? index : index + 1
}

const onPinnedZoneOver = () => {
  if (drag.value && insertAt.value === null) {
    insertAt.value = pinned.value.length
  }
}

const pinAt = (item: PatchResource, at: number) => {
  pinned.value.splice(at, 0, item)
}

const onDropPinned = () => {
  const d = drag.value
  if (!d) return
  let at = insertAt.value ?? pinned.value.length
  if (d.zone === 'pinned') {
    const [item] = pinned.value.splice(d.index, 1)
    if (d.index < at) at--
    pinAt(item!, at)
  } else {
    const [item] = rest.value.splice(d.index, 1)
    pinAt(item!, at)
  }
  onDragEnd()
}

const onDropRest = () => {
  const d = drag.value
  if (d?.zone === 'pinned') unpin(d.index)
  onDragEnd()
}

const move = (index: number, delta: number) => {
  const to = index + delta
  if (to < 0 || to >= pinned.value.length) return
  const [item] = pinned.value.splice(index, 1)
  pinned.value.splice(to, 0, item!)
}

const pin = (index: number) => {
  const [item] = rest.value.splice(index, 1)
  pinned.value.push(item!)
}

const unpin = (index: number) => {
  const [item] = pinned.value.splice(index, 1)
  rest.value.unshift(item!)
}

const clearPinned = () => {
  rest.value = [...pinned.value, ...rest.value]
  pinned.value = []
}

const saving = ref(false)
const save = async () => {
  saving.value = true
  const ids = pinned.value.map((r) => r.id)
  const res = await api.put(`/patch/${props.patchId}/resource/order`, {
    resource_ids: ids
  })
  saving.value = false
  if (res.code === 0) {
    useKunMessage('展示顺序已保存', 'success')
    emit('saved', ids)
  } else {
    useKunMessage(res.message || '保存失败', 'error')
  }
}

const storageLabel = (r: PatchResource) =>
  SUPPORTED_RESOURCE_LINK_MAP[r.storage] ?? r.storage
</script>

<template>
  <div class="space-y-4 py-2">
    <div>
      <h3 class="text-lg font-bold">调整资源展示顺序</h3>
      <p class="text-default-500 text-sm">
        拖动资源到「固定顺序」中即按该顺序置顶展示；其余资源每次打开页面时随机排列。
      </p>
    </div>

    <section class="space-y-2">
      <div class="flex items-center justify-between">
        <h4 class="flex items-center gap-1.5 text-sm font-semibold">
          <KunIcon name="lucide:pin" class="text-primary size-4" />
          固定顺序
        </h4>
        <KunButton
          v-if="pinned.length"
          variant="light"
          color="default"
          size="xs"
          @click="clearPinned"
        >
          全部取消固定
        </KunButton>
      </div>
      <div
        class="border-primary/30 min-h-16 space-y-1.5 rounded-xl border border-dashed p-2"
        @dragover.prevent="onPinnedZoneOver"
        @drop.prevent="onDropPinned"
      >
        <p
          v-if="!pinned.length"
          class="text-default-400 py-4 text-center text-sm"
        >
          把资源拖到这里，或点击下方资源的「固定」
        </p>
        <div
          v-for="(r, i) in pinned"
          :key="r.id"
          draggable="true"
          :class="pinnedRowClass(i)"
          @dragstart="(e) => onDragStart('pinned', i, e)"
          @dragend="onDragEnd"
          @dragover.prevent.stop="(e) => onPinnedRowOver(i, e)"
        >
          <KunIcon
            name="lucide:grip-vertical"
            class="text-default-400 size-4 shrink-0"
          />
          <span
            class="text-primary w-5 shrink-0 text-center text-sm font-semibold"
          >
            {{ i + 1 }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-medium">
              {{ r.name || '补丁资源' }}
            </div>
            <div class="text-default-500 text-xs">
              {{ storageLabel(r) }} · {{ r.size }} ·
              {{ r.user?.name ?? '未知用户' }}
            </div>
          </div>
          <KunButton
            is-icon-only
            variant="light"
            color="default"
            size="xs"
            aria-label="上移"
            :disabled="i === 0"
            @click="move(i, -1)"
          >
            <KunIcon name="lucide:chevron-up" class="size-4" />
          </KunButton>
          <KunButton
            is-icon-only
            variant="light"
            color="default"
            size="xs"
            aria-label="下移"
            :disabled="i === pinned.length - 1"
            @click="move(i, 1)"
          >
            <KunIcon name="lucide:chevron-down" class="size-4" />
          </KunButton>
          <KunButton
            is-icon-only
            variant="light"
            color="default"
            size="xs"
            aria-label="取消固定"
            @click="unpin(i)"
          >
            <KunIcon name="lucide:pin-off" class="size-4" />
          </KunButton>
        </div>
      </div>
    </section>

    <section class="space-y-2">
      <h4 class="flex items-center gap-1.5 text-sm font-semibold">
        <KunIcon name="lucide:clock" class="text-default-500 size-4" />
        按更新时间展示
      </h4>
      <div
        class="border-default/20 min-h-16 space-y-1.5 rounded-xl border border-dashed p-2"
        @dragover.prevent="insertAt = null"
        @drop.prevent="onDropRest"
      >
        <p
          v-if="!rest.length"
          class="text-default-400 py-4 text-center text-sm"
        >
          所有资源都已固定
        </p>
        <div
          v-for="(r, i) in rest"
          :key="r.id"
          draggable="true"
          :class="cn(ROW, isDragging('rest', i) && 'opacity-50')"
          @dragstart="(e) => onDragStart('rest', i, e)"
          @dragend="onDragEnd"
        >
          <KunIcon
            name="lucide:grip-vertical"
            class="text-default-400 size-4 shrink-0"
          />
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-medium">
              {{ r.name || '补丁资源' }}
            </div>
            <div class="text-default-500 text-xs">
              {{ storageLabel(r) }} · {{ r.size }} ·
              {{ r.user?.name ?? '未知用户' }}
            </div>
          </div>
          <KunButton variant="light" color="primary" size="xs" @click="pin(i)">
            <KunIcon name="lucide:pin" class="size-3.5" />
            固定
          </KunButton>
        </div>
      </div>
    </section>

    <div class="flex justify-end gap-2">
      <KunButton
        variant="light"
        color="default"
        :disabled="saving"
        @click="emit('close')"
      >
        取消
      </KunButton>
      <KunButton color="primary" :loading="saving" @click="save">
        <KunIcon name="lucide:check" class="size-4" />
        保存
      </KunButton>
    </div>
  </div>
</template>
