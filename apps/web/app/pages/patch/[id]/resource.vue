<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'
import { SUPPORTED_RESOURCE_LINK_MAP } from '~/constants/resource'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

const galgameId = computed(() => Number(route.params.id))

const sanitize = (html: string) =>
  DOMPurify.sanitize(html, { ADD_ATTR: ['data-uid'] })

const { data: resources, pending } = await useAsyncData<PatchResource[]>(
  () => `patch-resource-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchResource[]>(
      `/patch/${galgameId.value}/resource`
    )
    return res.code === 0 ? res.data : []
  },
  { default: () => [] }
)

// ─── Sorter ───────────────────────────────────────────
// Client-side (the list endpoint returns the whole set unpaginated).
type SortField = 'update_time' | 'created' | 'download' | 'like_count'
const sortField = ref<SortField>('update_time') // 更改时间
const sortDesc = ref(true) // 默认降序：最新更改在最上面

const sortOptions = [
  { value: 'update_time', label: '更改时间' },
  { value: 'created', label: '发布时间' },
  { value: 'download', label: '下载数' },
  { value: 'like_count', label: '点赞数' }
]

// `update_time` is the canonical 更改时间: the backend sets it = creation
// time on insert (gorm autoCreateTime) and explicitly bumps it to now() only
// when the resource is re-edited (UpdateResource). Do NOT use `updated`
// (gorm autoUpdateTime) — that also bumps on download/like increments, which
// would jerk rows around for non-edit activity. Mirrors next-api's
// `update_time: new Date()` on resource update.
const timeOf = (r: PatchResource, f: 'update_time' | 'created') => {
  const v = f === 'update_time' ? (r.update_time ?? r.created) : r.created
  return new Date(v as string).getTime()
}

const sortedResources = computed(() => {
  const list = [...(resources.value ?? [])]
  const f = sortField.value
  list.sort((a, b) => {
    const cmp =
      f === 'download' || f === 'like_count'
        ? (a[f] ?? 0) - (b[f] ?? 0)
        : timeOf(a, f) - timeOf(b, f)
    return sortDesc.value ? -cmp : cmp
  })
  return list
})

// ─── "获取资源链接" — fetch minimal link info, reveal inline ──
// Calls the lightweight GET /patch/resource/:id/link (only storage + links +
// secrets — no Wiki enrich / recommendations / blake3). A second click
// collapses. blake3 stays on the card; it's intentionally not duplicated here.
interface ResourceLinkInfo {
  storage: string
  content: string
  code: string
  password: string
}
const fetched = reactive<Record<number, ResourceLinkInfo>>({})
const loadingId = ref<number | null>(null)

const getResourceLink = async (r: PatchResource) => {
  if (fetched[r.id]) {
    delete fetched[r.id] // collapse
    return
  }
  loadingId.value = r.id
  try {
    const res = await api.get<ResourceLinkInfo>(
      `/patch/resource/${r.id}/link`
    )
    if (res.code === 0 && res.data) {
      fetched[r.id] = res.data
    } else {
      useKunMessage(res.message || '获取资源链接失败', 'error')
    }
  } finally {
    loadingId.value = null
  }
}

const linksOf = (d: ResourceLinkInfo) =>
  (d.content ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

const storageLabelOf = (d: ResourceLinkInfo) =>
  SUPPORTED_RESOURCE_LINK_MAP[d.storage] ?? d.storage

// Bump the download counter when a revealed link is actually clicked,
// mirroring the resource detail page.
const onLinkDownload = (r: PatchResource) => {
  api.put(`/patch/resource/${r.id}/download`).catch(() => {})
  r.download += 1
}

// Optimistic resource-like toggle, mirroring the comment pattern: backend
// returns { liked }, we fold it onto the local row.
const toggleLike = async (r: PatchResource) => {
  if (!userStore.user.uid) {
    useKunMessage('请先登录后再点赞', 'warn')
    return
  }
  const res = await api.put<{ liked: boolean }>(
    `/patch/resource/${r.id}/like`
  )
  if (res.code === 0) {
    const liked = res.data.liked
    const prev = r.is_liked ?? false
    const delta = liked === prev ? 0 : liked ? 1 : -1
    r.is_liked = liked
    r.like_count = Math.max(0, r.like_count + delta)
  } else {
    useKunMessage(res.message || '操作失败', 'error')
  }
}
</script>

<template>
  <div class="space-y-4">
    <!-- sorter -->
    <div
      v-if="!pending && resources && resources.length"
      class="border-default/20 bg-background flex flex-wrap items-center gap-2 rounded-2xl border p-3"
    >
      <KunIcon
        name="lucide:arrow-up-down"
        class="text-default-400 size-4"
      />
      <span class="text-default-500 text-sm">排序</span>
      <button
        v-for="o in sortOptions"
        :key="o.value"
        type="button"
        class="rounded-full px-3 py-1 text-sm transition-colors"
        :class="
          sortField === o.value
            ? 'bg-primary/10 text-primary font-medium'
            : 'text-default-500 hover:bg-default-100'
        "
        @click="sortField = o.value as SortField"
      >
        {{ o.label }}
      </button>
      <button
        type="button"
        class="text-default-500 hover:bg-default-100 ml-auto inline-flex items-center gap-1 rounded-full px-3 py-1 text-sm transition-colors"
        :title="sortDesc ? '降序' : '升序'"
        @click="sortDesc = !sortDesc"
      >
        <KunIcon
          :name="sortDesc ? 'lucide:arrow-down-wide-narrow' : 'lucide:arrow-up-narrow-wide'"
          class="size-4"
        />
        {{ sortDesc ? '降序' : '升序' }}
      </button>
    </div>

    <KunLoading v-if="pending" description="正在获取补丁资源..." />
    <div v-else-if="resources && resources.length" class="space-y-4">
      <div
        v-for="r in sortedResources"
        :key="r.id"
        class="border-default/20 bg-background hover:border-primary/40 space-y-4 rounded-2xl border p-5 transition-colors"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="text-lg font-semibold line-clamp-2">
              {{ r.name || '补丁资源' }}
            </h3>
            <div class="text-default-500 mt-1 flex items-center gap-2 text-xs">
              <KunAvatar :user="r.user" size="xs" />
              <span>
                由 <span class="text-foreground font-medium">{{
                  r.user.name
                }}</span> 发布于
                {{
                  formatDate(r.created, { isShowYear: true, isPrecise: true })
                }}
              </span>
            </div>
          </div>
          <KunChip color="warning" size="sm" variant="flat">
            <KunIcon name="lucide:database" class="size-3.5" />
            {{ r.size }}
          </KunChip>
        </div>

        <KunPatchAttribute
          :types="r.type"
          :languages="r.language"
          :platforms="r.platform"
          :model-name="r.model_name"
          :storage="r.storage"
          size="sm"
        />

        <div
          v-if="r.note_html"
          class="kun-prose border-default/15 bg-default-50 rounded-xl border p-3 text-sm"
          v-html="sanitize(r.note_html)"
        />

        <div v-if="r.code || r.password" class="flex flex-wrap gap-2">
          <KunCopy
            v-if="r.code"
            :text="r.code"
            :name="`提取码: ${r.code}`"
            color="secondary"
            variant="flat"
            size="sm"
          />
          <KunCopy
            v-if="r.password"
            :text="r.password"
            :name="`解压密码: ${r.password}`"
            color="secondary"
            variant="flat"
            size="sm"
          />
        </div>

        <div
          v-if="r.blake3"
          class="text-default-400 flex flex-wrap items-center gap-2 text-xs"
        >
          <span class="shrink-0">BLAKE3</span>
          <code
            class="bg-default-100 max-w-full truncate rounded-lg px-2 py-1"
          >
            {{ r.blake3 }}
          </code>
          <NuxtLink
            :to="`/check-hash?hash=${r.blake3}&content=${encodeURIComponent(r.content || '')}`"
          >
            <KunButton size="sm" variant="flat" color="primary" rounded="full">
              <KunIcon name="lucide:shield-check" class="size-3.5" />
              校验文件
            </KunButton>
          </NuxtLink>
        </div>

        <div
          class="border-default/15 flex flex-wrap items-center justify-between gap-2 border-t pt-3"
        >
          <div class="text-default-500 flex items-center gap-4 text-sm">
            <button
              type="button"
              :class="
                cn(
                  'flex items-center gap-1.5 transition-colors',
                  r.is_liked
                    ? 'text-danger'
                    : 'text-default-500 hover:text-danger'
                )
              "
              :aria-label="r.is_liked ? '取消点赞' : '点赞'"
              @click="toggleLike(r)"
            >
              <KunIcon
                name="lucide:heart"
                :class="cn('size-4', r.is_liked ? 'fill-current' : '')"
              />
              {{ r.like_count }}
            </button>
            <span class="flex items-center gap-1.5">
              <KunIcon name="lucide:download" class="size-4" />
              {{ r.download }}
            </span>
          </div>
          <KunButton
            color="primary"
            size="sm"
            rounded="full"
            :loading="loadingId === r.id"
            :disabled="loadingId === r.id"
            @click="getResourceLink(r)"
          >
            <KunIcon
              :name="fetched[r.id] ? 'lucide:chevron-up' : 'lucide:link'"
              class="size-4"
            />
            {{ fetched[r.id] ? '收起' : '获取资源链接' }}
          </KunButton>
        </div>

        <!-- inline reveal: download links + hash + secrets -->
        <div
          v-if="fetched[r.id]"
          class="border-success/40 bg-success/10 mt-1 space-y-3 rounded-xl border p-4"
        >
          <div class="flex items-center gap-2">
            <KunIcon
              name="lucide:download-cloud"
              class="text-success size-4"
            />
            <span class="text-sm font-semibold">资源下载链接</span>
            <KunChip color="secondary" variant="flat" size="sm">
              {{ storageLabelOf(fetched[r.id]!) }}
            </KunChip>
          </div>

          <div class="space-y-2">
            <div
              v-for="(lnk, i) in linksOf(fetched[r.id]!)"
              :key="i"
              class="border-success/40 bg-background hover:border-success flex items-center gap-3 rounded-lg border p-2.5"
            >
              <a
                :href="lnk"
                target="_blank"
                rel="noopener noreferrer"
                class="hover:text-success group flex min-w-0 flex-1 items-center gap-2 transition-colors"
                @click="onLinkDownload(r)"
              >
                <KunIcon
                  name="lucide:download"
                  class="text-success size-4 shrink-0"
                />
                <span class="min-w-0 flex-1 truncate text-sm">{{ lnk }}</span>
                <KunIcon
                  name="lucide:external-link"
                  class="text-default-400 group-hover:text-success size-3.5 shrink-0"
                />
              </a>
              <button
                type="button"
                class="text-default-400 hover:text-success hover:bg-success/10 shrink-0 rounded-md p-1.5 transition-colors"
                aria-label="复制下载链接"
                title="复制下载链接"
                @click="useKunCopy(lnk)"
              >
                <KunIcon name="lucide:copy" class="size-4" />
              </button>
            </div>
            <p
              v-if="!linksOf(fetched[r.id]!).length"
              class="text-default-500 text-sm"
            >
              暂无可用下载链接
            </p>
          </div>

          <div
            v-if="fetched[r.id]!.code || fetched[r.id]!.password"
            class="flex flex-wrap gap-2"
          >
            <KunCopy
              v-if="fetched[r.id]!.code"
              :text="fetched[r.id]!.code!"
              :name="`提取码: ${fetched[r.id]!.code}`"
              color="secondary"
              variant="flat"
              size="sm"
            />
            <KunCopy
              v-if="fetched[r.id]!.password"
              :text="fetched[r.id]!.password!"
              :name="`解压密码: ${fetched[r.id]!.password}`"
              color="secondary"
              variant="flat"
              size="sm"
            />
          </div>
        </div>
      </div>
    </div>
    <KunNull v-else description="该 Galgame 暂无补丁资源" />
  </div>
</template>
