<script setup lang="ts">
import { blake3 } from '@noble/hashes/blake3.js'
import { bytesToHex } from '@noble/hashes/utils.js'

const route = useRoute()
const router = useRouter()

const hashValue = ref(String(route.query.hash ?? ''))
const contentValue = computed(() => String(route.query.content ?? ''))
const status = ref<'idle' | 'checking' | 'match' | 'mismatch'>('idle')
const progress = ref(0)
const isDragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

watch(
  () => route.query.hash,
  (v) => {
    hashValue.value = String(v ?? '')
  }
)

const updateHash = (value: string) => {
  hashValue.value = value
  router.push(value ? `/check-hash?hash=${value}` : '/check-hash')
}

const verifyFile = async (file: File) => {
  status.value = 'checking'
  progress.value = 0

  const chunkSize = 64 * 1024
  const fileSize = file.size
  const hashInstance = blake3.create({})
  let bytesProcessed = 0

  const processChunk = async (start: number) => {
    const end = Math.min(start + chunkSize, fileSize)
    const chunk = await file.slice(start, end).arrayBuffer()
    const uint8 = new Uint8Array(chunk)

    hashInstance.update(uint8)
    bytesProcessed += uint8.byteLength
    progress.value = Math.round((bytesProcessed / fileSize) * 100)

    if (end < fileSize) {
      await new Promise((resolve) => setTimeout(resolve, 0))
      await processChunk(end)
    } else {
      const hash = bytesToHex(hashInstance.digest())
      status.value =
        hash.toLowerCase() === hashValue.value.toLowerCase()
          ? 'match'
          : 'mismatch'
    }
  }

  try {
    await processChunk(0)
  } catch (err) {
    useKunMessage(`校验文件错误! ${err}`, 'error')
    status.value = 'mismatch'
  }
}

const handleDrop = (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = false
  const file = e.dataTransfer?.files[0]
  if (file) verifyFile(file)
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = true
}

const handleDragLeave = (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = false
}

const handleFileSelect = (e: Event) => {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) verifyFile(file)
}
</script>

<template>
  <KunCard class-name="mx-auto max-w-2xl p-8" :bordered="false">
    <template #header>
      <div class="flex flex-col space-y-6">
        <div v-if="contentValue" class="w-full">
          <KunTextarea
            label="资源链接"
            :model-value="contentValue"
            readonly
            :rows="3"
          />
        </div>
        <KunInput
          label="BLAKE3 Hash"
          size="lg"
          color="primary"
          :model-value="hashValue"
          helper-text="您可以输入文件 BLAKE3 Hash 值以进行校验 (如果您是从本站跳转, 本站会为您自动补全 Hash)"
          @update:model-value="updateHash(String($event))"
        />
      </div>
    </template>

    <div>
      <div
        :class="
          cn(
            'mb-4 rounded-lg border-2 border-dashed p-4 text-center transition-colors',
            isDragging ? 'border-primary bg-primary/10' : 'border-default-300'
          )
        "
        @drop="handleDrop"
        @dragover="handleDragOver"
        @dragleave="handleDragLeave"
      >
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          @change="handleFileSelect"
        />
        <KunIcon name="lucide:file" class="text-default-400 mx-auto mb-4 size-12" />
        <p class="mb-2">拖动或点击以上传文件</p>
        <KunButton color="primary" variant="flat" @click="fileInput?.click()">
          选择文件
        </KunButton>
      </div>

      <div v-if="status === 'checking'" class="mt-6">
        <KunProgress :value="progress" color="primary" />
        <p class="text-default-500 mt-2 text-center">
          正在校验文件 Hash... {{ progress }}%
        </p>
      </div>

      <div
        v-else-if="status === 'match'"
        class="text-success mt-6 flex items-center justify-center gap-2"
      >
        <KunIcon name="lucide:circle-check" class="size-6" />
        <span>校验成功! Hash 一致, 文件未损坏🎉🎉🎉</span>
      </div>

      <div
        v-else-if="status === 'mismatch'"
        class="text-danger mt-6 flex items-center justify-center gap-2"
      >
        <KunIcon name="lucide:circle-x" class="size-6" />
        <span>校验失败! 文件可能在下载过程中损坏!</span>
      </div>
    </div>
  </KunCard>
</template>
