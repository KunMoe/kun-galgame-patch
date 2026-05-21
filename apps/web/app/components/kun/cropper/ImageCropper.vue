<script setup lang="ts">
import type Cropper from 'cropperjs'

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

const handleConfirm = async () => {
  if (!cropperInstance) {
    showCropper.value = false
    return
  }
  const selection = cropperInstance.getCropperSelection()
  if (!selection) return

  try {
    const canvas = await selection.$toCanvas()
    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob((b) => resolve(b), 'image/webp', 0.9)
    })
    if (!blob) {
      useKunMessage('裁剪失败', 'error')
      return
    }
    if (previewUrl.value && !previewUrl.value.startsWith('http')) {
      URL.revokeObjectURL(previewUrl.value)
    }
    previewUrl.value = URL.createObjectURL(blob)
    emits('complete', blob)
  } finally {
    showCropper.value = false
    if (cropperSrc.value) {
      URL.revokeObjectURL(cropperSrc.value)
      cropperSrc.value = ''
    }
  }
}

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
        <div class="flex justify-end gap-2">
          <KunButton variant="light" color="danger" @click="handleCancel">
            取消
          </KunButton>
          <KunButton color="primary" @click="handleConfirm">
            确定裁剪
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
