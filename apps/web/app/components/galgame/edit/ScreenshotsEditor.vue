<script setup lang="ts">
// W2 / PR3b — manage the screenshots array of a galgame (image_service hashes).
//
// Upload flow: file picker → POST /api/v1/upload/image-service (preset='topic',
// moyu's default-enabled preset) → returns content hash → append to local
// array with the next sort_order. Parent (rewrite.vue / prs.vue) saves the
// full array via PUT /galgame/:gid (presence-replace semantics; only when the
// set actually changed).
//
// Per docs/galgame_wiki/03-relations.md screenshots have NO multipart direct
// upload via Wiki — clients MUST first take the image_service detour.

import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  modelValue: GalgameScreenshotRow[]
}>()
const emit = defineEmits<{
  'update:modelValue': [GalgameScreenshotRow[]]
}>()

const ge = useGalgameEdit()

// Display order = sort_order ascending; stable on ties.
const sorted = computed(() =>
  [...props.modelValue].sort((a, b) => a.sort_order - b.sort_order)
)

// Upload one or more files; each appended to the array at the next sort_order.
const uploading = ref(false)
const pickedFiles = ref<File[]>([])

const handleFiles = async (files: File[]) => {
  if (!files.length) return
  uploading.value = true
  try {
    const nextSortOrder = props.modelValue.length
      ? Math.max(...props.modelValue.map((s) => s.sort_order)) + 1
      : 0
    const additions: GalgameScreenshotRow[] = []
    let i = 0
    for (const f of files) {
      const res = await ge.uploadImageService(f, 'topic')
      if (res.code !== 0 || !res.data) {
        useKunMessage(
          `上传 ${f.name} 失败：${res.message || '未知错误'}`,
          'error'
        )
        continue
      }
      // Skip if hash is already in the list (image_service dedupes; would
      // create a duplicate row that Wiki rejects on primary key).
      if (
        props.modelValue.some((s) => s.image_hash === res.data!.hash) ||
        additions.some((s) => s.image_hash === res.data!.hash)
      ) {
        useKunMessage(`已存在相同截图：${f.name}（跳过）`, 'warn')
        continue
      }
      additions.push({
        image_hash: res.data.hash,
        sort_order: nextSortOrder + i,
        caption: '',
        sexual: 0,
        violence: 0,
        source: '',
        source_key: ''
      })
      i++
    }
    if (additions.length) {
      emit('update:modelValue', [...props.modelValue, ...additions])
      useKunMessage(`已上传 ${additions.length} 张截图`, 'success')
    }
  } finally {
    uploading.value = false
    // Clear KunFileInput's selection so picking the same file again still
    // triggers @change (matches native input reset semantics).
    pickedFiles.value = []
  }
}

const setCaption = (hash: string, caption: string) => {
  emit(
    'update:modelValue',
    props.modelValue.map((s) =>
      s.image_hash === hash ? { ...s, caption } : s
    )
  )
}

// Simple reorder via up/down: swap sort_order with the neighbor in the sorted
// list. Drag-reorder is fancier but adds significant complexity for marginal
// gain in admin-rare workflows.
const move = (hash: string, dir: -1 | 1) => {
  const list = sorted.value
  const idx = list.findIndex((s) => s.image_hash === hash)
  const swap = idx + dir
  if (idx < 0 || swap < 0 || swap >= list.length) return
  // idx/swap are bounds-checked above, so both are in range.
  const a = list[idx]!
  const b = list[swap]!
  emit(
    'update:modelValue',
    props.modelValue.map((s) => {
      if (s.image_hash === a.image_hash) return { ...s, sort_order: b.sort_order }
      if (s.image_hash === b.image_hash) return { ...s, sort_order: a.sort_order }
      return s
    })
  )
}

const remove = async (hash: string) => {
  const ok = await useKunAlert({
    title: '移除截图',
    message: '确定移除该截图？将在保存后从 Wiki 集合里删除。'
  })
  if (!ok) return
  emit(
    'update:modelValue',
    props.modelValue.filter((s) => s.image_hash !== hash)
  )
}
</script>

<template>
  <div class="border-default/20 space-y-3 rounded-xl border p-3">
    <div class="flex items-center justify-between">
      <p class="text-foreground text-sm font-semibold">截图 / 画廊</p>
      <KunFileInput
        v-model:files="pickedFiles"
        multiple
        accept="image/jpeg,image/png,image/webp"
        :disabled="uploading"
        :show-file-name="false"
        trigger-text="上传截图"
        trigger-icon="lucide:image-plus"
        trigger-size="sm"
        @change="handleFiles"
      />
    </div>
    <p class="text-default-500 text-xs">
      可一次选多张；按选择顺序追加到列表末尾。每张图最大 10MB。
    </p>
    <p v-if="!sorted.length" class="text-default-400 text-xs">暂无截图</p>
    <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div
        v-for="(s, idx) in sorted"
        :key="s.image_hash"
        class="border-default/20 rounded-lg border p-2"
      >
        <KunImage
          :src="imageServiceUrl(s.image_hash)"
          :alt="s.caption || s.image_hash.slice(0, 8)"
          aspect-ratio="16 / 9"
          class-name="bg-default-100 rounded"
        />
        <KunInput
          :model-value="s.caption"
          placeholder="可选 caption（短描述）"
          size="sm"
          class-name="mt-2"
          @update:model-value="setCaption(s.image_hash, String($event))"
        />
        <div class="mt-2 flex items-center justify-between">
          <span class="text-default-400 text-xs">#{{ idx + 1 }}</span>
          <div class="flex gap-1">
            <KunButton
              variant="light"
              size="sm"
              :disabled="idx === 0"
              @click="move(s.image_hash, -1)"
            >
              ↑
            </KunButton>
            <KunButton
              variant="light"
              size="sm"
              :disabled="idx === sorted.length - 1"
              @click="move(s.image_hash, 1)"
            >
              ↓
            </KunButton>
            <KunButton
              variant="light"
              color="danger"
              size="sm"
              @click="remove(s.image_hash)"
            >
              移除
            </KunButton>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
