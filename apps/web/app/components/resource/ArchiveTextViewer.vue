<script setup lang="ts">
import type { ArchiveNode } from '~/shared/utils/archiveTree'
import type { TextEncoding } from '~/shared/utils/textEncoding'

const props = defineProps<{
  file: ArchiveNode
  src: string
  truncated: boolean
  maxBytes: number
  decodedSave?: boolean
}>()

const open = defineModel<boolean>({ required: true })

const bytes = shallowRef<Uint8Array | null>(null)
const error = ref('')
const encoding = ref<TextEncoding>('utf-8')
const wrap = ref(true)
const showBinary = ref(false)
const cutByServer = ref(false)

const load = async () => {
  bytes.value = null
  error.value = ''
  showBinary.value = false
  try {
    const res = await fetch(props.src, { credentials: 'include' })
    if (!res.ok) {
      error.value = await readFailure(res)
      return
    }
    const body = new Uint8Array(await res.arrayBuffer())
    cutByServer.value = res.headers.get('x-content-truncated') === 'true'
    encoding.value = props.decodedSave ? 'utf-8' : await detectEncoding(body)
    bytes.value = body
  } catch {
    error.value = '网络错误，请稍后再试'
  }
}

watch(() => props.src, load, { immediate: true })

const text = computed(() =>
  bytes.value ? decodeText(bytes.value, encoding.value) : ''
)
const binary = computed(
  () =>
    !!bytes.value &&
    !showBinary.value &&
    looksBinary(bytes.value, encoding.value)
)
const folder = computed(() => props.file.parent || '/')
const viewName = computed(() =>
  props.decodedSave ? `${props.file.name}.json` : props.file.name
)
</script>

<template>
  <KunModal
    v-model="open"
    :title="file.name"
    size="xl"
    inner-class-name="w-full max-w-5xl"
  >
    <div class="flex min-w-0 flex-col gap-3">
      <div
        class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-xs"
      >
        <p class="text-default-400 min-w-0 truncate" :title="file.path">
          {{ folder }} · {{ formatBytes(file.size) }}
        </p>
        <div class="flex items-center gap-3">
          <KunSwitch v-model="wrap" size="sm" label="自动换行" />
          <KunSelect
            v-if="!decodedSave"
            v-model="encoding"
            class-name="w-32"
            :options="TEXT_ENCODINGS"
            size="sm"
            aria-label="文本编码"
          />
        </div>
      </div>

      <p v-if="decodedSave" class="text-default-400 text-xs">
        RPG Maker 存档，已解码为 JSON 显示。
      </p>
      <p v-if="truncated && bytes" class="text-warning text-xs">
        文件较大，这里只显示前
        {{ formatBytes(maxBytes) }}，完整内容请下载后查看。
      </p>
      <p v-if="cutByServer" class="text-warning text-xs">
        存档解码后内容较多，这里只显示开头部分，完整内容请下载后查看。
      </p>

      <div
        class="border-default-200 rounded-kun-md h-[min(65dvh,44rem)] overflow-hidden border"
      >
        <div
          v-if="error"
          class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-sm"
        >
          <p class="text-danger">{{ error }}</p>
          <KunButton size="sm" variant="flat" @click="load">重试</KunButton>
        </div>
        <div
          v-else-if="!bytes"
          class="text-default-400 flex h-full items-center justify-center gap-2 text-sm"
        >
          <KunIcon name="lucide:loader-circle" class="size-4 animate-spin" />
          正在读取文件
        </div>
        <div
          v-else-if="binary"
          class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-sm"
        >
          <p class="text-default-500">
            这个文件看起来不是纯文本，强行显示可能是乱码。
          </p>
          <KunButton
            size="sm"
            variant="flat"
            color="default"
            @click="showBinary = true"
          >
            仍然显示
          </KunButton>
        </div>
        <ResourceArchiveCodeView
          v-else
          :text="text"
          :file-name="viewName"
          :wrap="wrap"
        />
      </div>
    </div>
  </KunModal>
</template>
