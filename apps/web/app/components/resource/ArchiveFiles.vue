<script setup lang="ts">
import type {
  ArchiveEntry,
  ArchiveListing,
  ArchiveNode
} from '~/shared/utils/archiveTree'
import type {
  ArchivePreview,
  ArchivePreviewRule
} from '~/shared/utils/archivePreview'

interface ResourceArchiveData {
  listing: ArchiveListing | null
  preview: ArchivePreviewRule[]
}

const props = defineProps<{ resourceId: number }>()

const api = useApi()

const { data } = useLazyAsyncData(
  () => `resource-files-${props.resourceId}`,
  async () => {
    const res = await api.get<ResourceArchiveData>(
      `/resource/${props.resourceId}/files`
    )
    return res.code === 0 ? res.data : null
  }
)

const listing = computed(() => data.value?.listing ?? null)
const ready = computed(() =>
  listing.value?.state === 'ready' ? listing.value : null
)
const pending = computed(() => listing.value?.state === 'pending')

const count = (n: number) => n.toLocaleString('zh-CN')

const full = shallowRef<ArchiveEntry[] | null>(null)
const loadingFull = ref(false)
const entries = computed(() => full.value ?? ready.value?.entries ?? [])

const loadFull = async () => {
  if (!ready.value?.truncated || loadingFull.value || full.value) return
  loadingFull.value = true
  const res = await api.get<ResourceArchiveData>(
    `/resource/${props.resourceId}/files?full=1`
  )
  loadingFull.value = false
  if (res.code !== 0 || !res.data.listing) {
    useKunMessage('压缩包目录加载失败，请稍后重试', 'warn')
    return
  }
  full.value = res.data.listing.entries
}

const resolve = computed(() =>
  createPreviewResolver(data.value?.preview ?? [])
)
const canOpen = (node: ArchiveNode) => !!resolve.value(node)
const srcOf = (node: ArchiveNode) =>
  api.url(archiveFileEndpoint(props.resourceId, node.path))

const GALLERY_RADIUS = 30

const viewing = shallowRef<{
  node: ArchiveNode
  preview: ArchivePreview
} | null>(null)
const viewerOpen = ref(false)
const gallery = shallowRef<{ src: string; alt: string }[]>([])
const galleryIndex = ref(0)
const galleryOpen = ref(false)
const galleryKey = ref(0)

const openImage = async (node: ArchiveNode, siblings: ArchiveNode[]) => {
  const images = siblings.filter((n) => resolve.value(n)?.kind === 'image')
  const at = Math.max(0, images.indexOf(node))
  const from = Math.max(0, at - GALLERY_RADIUS)
  gallery.value = images
    .slice(from, at + GALLERY_RADIUS + 1)
    .map((n) => ({ src: srcOf(n), alt: n.name }))
  galleryIndex.value = at - from
  galleryOpen.value = false
  galleryKey.value++
  await nextTick()
  galleryOpen.value = true
}

const openFile = (node: ArchiveNode, siblings: ArchiveNode[]) => {
  const preview = resolve.value(node)
  if (!preview) return
  if (preview.kind === 'image') {
    openImage(node, siblings)
    return
  }
  viewing.value = { node, preview }
  viewerOpen.value = true
}
</script>

<template>
  <section v-if="ready" class="space-y-3">
    <div class="min-w-0">
      <h2 class="text-lg font-semibold">压缩包内容</h2>
      <p class="text-default-400 mt-0.5 text-xs">
        {{ count(ready.file_count) }} 个文件 ·
        {{ count(ready.dir_count) }} 个文件夹 · 解压后
        {{ formatBytes(ready.total_bytes) }}
      </p>
      <p class="text-default-400 mt-0.5 text-xs">
        文件名以 UTF-8 存储，部分系统自带的解压工具会解成乱码，推荐用 7-Zip 或
        Bandizip 解压。文本、图片、音频和 RPG Maker 存档可以直接点开查看。
      </p>
    </div>
    <ResourceArchiveTree
      :entries="entries"
      :can-open="canOpen"
      :loading="loadingFull"
      @open="openFile"
      @load="loadFull"
    />

    <template v-if="viewing">
      <ResourceArchiveTextViewer
        v-if="viewing.preview.kind === 'text' || viewing.preview.kind === 'save'"
        v-model="viewerOpen"
        :file="viewing.node"
        :src="srcOf(viewing.node)"
        :truncated="viewing.preview.truncated"
        :max-bytes="viewing.preview.maxBytes"
        :decoded-save="viewing.preview.kind === 'save'"
      />
      <ResourceArchiveAudioViewer
        v-else
        v-model="viewerOpen"
        :file="viewing.node"
        :src="srcOf(viewing.node)"
      />
    </template>
    <KunLightbox
      v-if="gallery.length"
      :key="galleryKey"
      v-model:is-open="galleryOpen"
      :images="gallery"
      :initial-index="galleryIndex"
    />
  </section>

  <section v-else-if="pending">
    <h2 class="mb-1 text-lg font-semibold">压缩包内容</h2>
    <p class="text-default-400 flex items-center gap-2 text-sm">
      <KunIcon name="lucide:loader-circle" class="size-4 animate-spin" />
      正在读取压缩包目录，稍后刷新页面即可查看。
    </p>
  </section>
</template>
