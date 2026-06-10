<script setup lang="ts">
import type Cropper from 'cropperjs'
import ImageMosaic from './ImageMosaic.vue'

interface Props {
  aspectRatio?: number
  initialImage?: string
  hint?: string
  description?: string
  className?: string
}

const props = withDefaults(defineProps<Props>(), {
  aspectRatio: 16 / 9,
  initialImage: '',
  hint: '点击或拖放图片到此处',
  description: '',
  className: ''
})

const emits = defineEmits<{
  complete: [blob: Blob]
  remove: []
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const imgRef = ref<HTMLImageElement | null>(null)
const previewUrl = ref<string>(props.initialImage)
const showCropper = ref(false)
const cropperSrc = ref('')

let cropperInstance: Cropper | null = null

watch(
  () => props.initialImage,
  (v) => {
    if (!previewUrl.value) previewUrl.value = v
  }
)

const destroyCropper = () => {
  if (cropperInstance) {
    cropperInstance.destroy()
    cropperInstance = null
  }
}

const openCropperWithFile = (file: File) => {
  if (!file.type.startsWith('image/')) {
    useKunMessage('请选择图片文件', 'error')
    return
  }
  if (cropperSrc.value) URL.revokeObjectURL(cropperSrc.value)
  cropperSrc.value = URL.createObjectURL(file)
  showCropper.value = true
}

watch(showCropper, async (open) => {
  if (!import.meta.client) return
  if (open) {
    await nextTick()
    if (imgRef.value) {
      destroyCropper()
      const mod = await import('cropperjs')
      const CropperClass = mod.default
      cropperInstance = new CropperClass(imgRef.value, {})
      const selection = cropperInstance.getCropperSelection()
      if (selection) {
        selection.aspectRatio = props.aspectRatio
        selection.initialAspectRatio = props.aspectRatio
      }
    }
  } else {
    destroyCropper()
  }
})

const handleFileChange = (e: Event) => {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) openCropperWithFile(file)
  target.value = ''
}

const handleDrop = (e: DragEvent) => {
  e.preventDefault()
  const file = e.dataTransfer?.files?.[0]
  if (file) openCropperWithFile(file)
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
}

const handleCancel = () => {
  showCropper.value = false
  if (cropperSrc.value) {
    URL.revokeObjectURL(cropperSrc.value)
    cropperSrc.value = ''
  }
}

const cropToBlob = async (): Promise<Blob | null> => {
  if (!cropperInstance) return null
  const selection = cropperInstance.getCropperSelection()
  if (!selection) return null
  const canvas = await selection.$toCanvas()
  // Bound the upload: cap the width to 1920 (16:9 covers → ≤1920×1080),
  // downscaling only (never upscaling a smaller crop) so we don't blur. Mirrors
  // the legacy's fixed cover size; image_service still derives display variants.
  let out = canvas
  if (canvas.width > 1920) {
    out = document.createElement('canvas')
    out.width = 1920
    out.height = Math.round((1920 * canvas.height) / canvas.width)
    out.getContext('2d')?.drawImage(canvas, 0, 0, out.width, out.height)
  }
  return new Promise<Blob | null>((resolve) => {
    out.toBlob((b) => resolve(b), 'image/webp', 0.9)
  })
}

const applyResult = (blob: Blob) => {
  if (previewUrl.value && !previewUrl.value.startsWith('http')) {
    URL.revokeObjectURL(previewUrl.value)
  }
  previewUrl.value = URL.createObjectURL(blob)
  emits('complete', blob)
}

const closeCropper = () => {
  showCropper.value = false
  if (cropperSrc.value) {
    URL.revokeObjectURL(cropperSrc.value)
    cropperSrc.value = ''
  }
}

// 完成裁剪 — use the crop as-is (skip 打码).
const handleConfirm = async () => {
  const blob = await cropToBlob()
  if (!blob) {
    useKunMessage('裁剪失败', 'error')
    return
  }
  applyResult(blob)
  closeCropper()
}

// ─── 打码 (mosaic) — optional step after cropping ─────────────────────
const showMosaic = ref(false)
const mosaicSrc = ref('')

// 裁剪并打码 — crop, then open the mosaic editor on the cropped result.
const handleCropThenMosaic = async () => {
  const blob = await cropToBlob()
  if (!blob) {
    useKunMessage('裁剪失败', 'error')
    return
  }
  if (mosaicSrc.value) URL.revokeObjectURL(mosaicSrc.value)
  mosaicSrc.value = URL.createObjectURL(blob)
  closeCropper()
  showMosaic.value = true
}

const onMosaicComplete = (blob: Blob) => {
  applyResult(blob)
}

// Revoke the mosaic source when the mosaic modal closes (complete OR cancel).
watch(showMosaic, (v) => {
  if (!v && mosaicSrc.value) {
    URL.revokeObjectURL(mosaicSrc.value)
    mosaicSrc.value = ''
  }
})

const handleRemove = () => {
  if (previewUrl.value && !previewUrl.value.startsWith('http')) {
    URL.revokeObjectURL(previewUrl.value)
  }
  previewUrl.value = ''
  emits('remove')
}

onUnmounted(() => {
  destroyCropper()
  if (cropperSrc.value) URL.revokeObjectURL(cropperSrc.value)
  if (mosaicSrc.value) URL.revokeObjectURL(mosaicSrc.value)
})
</script>

<template>
  <div>
    <div
      :class="
        cn(
          'border-default-500 hover:border-default-700 bg-default-50 relative cursor-pointer rounded-lg border-2 border-dashed transition-colors',
          props.className
        )
      "
      :style="{ aspectRatio: props.aspectRatio }"
      @drop="handleDrop"
      @dragover="handleDragOver"
      @click="fileInput?.click()"
    >
      <img
        v-if="previewUrl"
        :src="previewUrl"
        alt="preview"
        class="h-full w-full rounded-lg object-cover"
      />
      <div
        v-else
        class="text-default-500 absolute inset-0 flex flex-col items-center justify-center"
      >
        <KunIcon name="lucide:image-plus" class="size-10" />
        <span v-if="hint" class="mt-2 text-sm">{{ hint }}</span>
      </div>
      <!-- Internal hidden <input>: this component encapsulates a drop zone +
           cropper + replace button, and all three need to trigger the same
           single picker via fileInput.click(). KunFileInput's `pick` is only
           exposed through its default slot, so it can't be shared with the
           re-select KunButton living outside the slot. Same pattern KunFile-
           Input itself uses internally (via useFilePicker). -->
      <input
        ref="fileInput"
        type="file"
        accept="image/*"
        class="hidden"
        @change="handleFileChange"
      />
    </div>

    <p v-if="description" class="text-default-500 mt-2 text-xs">
      {{ description }}
    </p>

    <div v-if="previewUrl" class="mt-2 flex gap-2">
      <KunButton variant="light" color="primary" @click="fileInput?.click()">
        <KunIcon name="lucide:refresh-cw" class="size-4" />
        重新选择
      </KunButton>
      <KunButton variant="light" color="danger" @click="handleRemove">
        <KunIcon name="lucide:trash-2" class="size-4" />
        删除
      </KunButton>
    </div>

    <KunModal
      :model-value="showCropper"
      inner-class-name="max-w-3xl w-full"
      :is-dismissable="false"
      @update:model-value="(v) => !v && handleCancel()"
    >
      <div class="space-y-4">
        <h3 class="text-lg font-semibold">裁剪图片</h3>
        <div class="max-h-[60vh] overflow-auto">
          <img
            v-if="cropperSrc"
            ref="imgRef"
            :src="cropperSrc"
            alt="cropper source"
            style="display: block; max-width: 100%"
          />
        </div>
        <div class="flex flex-wrap justify-end gap-2">
          <KunButton variant="light" color="danger" @click="handleCancel">
            取消
          </KunButton>
          <KunButton
            variant="flat"
            color="secondary"
            @click="handleCropThenMosaic"
          >
            <KunIcon name="lucide:grid-2x2" class="size-4" />
            裁剪并打码
          </KunButton>
          <KunButton color="primary" @click="handleConfirm">
            完成裁剪
          </KunButton>
        </div>
      </div>
    </KunModal>

    <ImageMosaic
      v-model:open="showMosaic"
      :src="mosaicSrc"
      @complete="onMosaicComplete"
    />
  </div>
</template>
