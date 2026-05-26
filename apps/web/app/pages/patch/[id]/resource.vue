<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'
import { SUPPORTED_RESOURCE_LINK_MAP } from '~/constants/resource'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

const galgameId = computed(() => Number(route.params.id))

const sanitize = (html: string) =>
  DOMPurify.sanitize(html, { ADD_ATTR: ['data-id'] })

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

// ─── 发布资源 (modal) ─────────────────────────────
// Login-gated entry; AuthEntry modal already lives in top-bar so we don't
// duplicate it here — just nudge the user to log in.
const publishOpen = ref(false)
const handlePublishClick = () => {
  if (!userStore.isLoggedIn) {
    useKunMessage('请先登录后再发布资源', 'warn')
    return
  }
  publishOpen.value = true
}
const handlePublishSuccess = (created: PatchResource) => {
  // Optimistic prepend so the new row appears immediately. The list is
  // unpaginated so we don't need to refetch.
  if (resources.value) {
    resources.value = [created, ...resources.value]
  }
}

// ─── 编辑 / 删除 (owner / moderator) ────────────────
// Front-end gates the button so non-owners don't see the affordance, but the
// server's PatchService.UpdateResource + DeleteResource enforce the same
// predicate (`UserID == caller || isPrivileged`) — UI is for noise reduction
// only, not security.
const canEdit = (r: PatchResource) =>
  userStore.isModerator || r.user_id === userStore.user.id
const canDelete = (r: PatchResource) =>
  userStore.isModerator || r.user_id === userStore.user.id

// Edit modal state. ResourcePublish handles both create + edit via the
// `resource` prop; success returns a merged row we splice into the list.
const editOpen = ref(false)
const editingResource = ref<PatchResource | null>(null)
const askEdit = (r: PatchResource) => {
  editingResource.value = r
  editOpen.value = true
}
const handleEditSuccess = (updated: PatchResource) => {
  if (!resources.value) return
  // Server returns the canonical, fully-rendered row — drop it straight in.
  // No more hand-rolled spread merge: the previous version kept old fields
  // the form doesn't send (e.g. update_time was stale), which surfaced as
  // "edited row doesn't bubble to top" + "description stuck on old html".
  resources.value = resources.value.map((r) =>
    r.id === updated.id ? updated : r
  )
}

// True when the row has been edited at least once. Backend stamps
// UpdateTime = time.Now() on every UpdateResource and leaves it equal to
// `created` on insert, so a string diff is a reliable "ever edited" signal.
// Both fields may arrive as Date | string; normalize via getTime for safety.
const hasBeenEdited = (r: PatchResource) => {
  if (!r.update_time) return false
  const u = new Date(r.update_time as string | Date).getTime()
  const c = new Date(r.created).getTime()
  // Same-millisecond inserts have u === c; treat anything > 1s apart as edit
  // to avoid a millisecond-jitter false positive on freshly-inserted rows.
  return Number.isFinite(u) && Number.isFinite(c) && u - c > 1000
}

const deleteOpen = ref(false)
const deleting = ref(false)
const pendingDelete = ref<PatchResource | null>(null)

const askDelete = (r: PatchResource) => {
  pendingDelete.value = r
  deleteOpen.value = true
}

const confirmDelete = async () => {
  const r = pendingDelete.value
  if (!r) return
  deleting.value = true
  try {
    const res = await api.delete(`/patch/resource/${r.id}`)
    if (res.code === 0) {
      useKunMessage('已删除资源', 'success')
      if (resources.value) {
        resources.value = resources.value.filter((x) => x.id !== r.id)
      }
      deleteOpen.value = false
      pendingDelete.value = null
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    deleting.value = false
  }
}

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
  if (!userStore.user.id) {
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
    <!-- 发布按钮 -->
    <div class="flex justify-end">
      <KunButton color="primary" @click="handlePublishClick">
        <KunIcon name="lucide:plus" class="size-4" />
        发布资源
      </KunButton>
    </div>

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
      <KunButton
        v-for="o in sortOptions"
        :key="o.value"
        :variant="sortField === o.value ? 'flat' : 'light'"
        :color="sortField === o.value ? 'primary' : 'default'"
        size="sm"
        rounded="full"
        @click="sortField = o.value as SortField"
      >
        {{ o.label }}
      </KunButton>
      <KunButton
        variant="light"
        color="default"
        size="sm"
        rounded="full"
        class-name="ml-auto"
        :title="sortDesc ? '降序' : '升序'"
        @click="sortDesc = !sortDesc"
      >
        <KunIcon
          :name="sortDesc ? 'lucide:arrow-down-wide-narrow' : 'lucide:arrow-up-narrow-wide'"
          class="size-4"
        />
        {{ sortDesc ? '降序' : '升序' }}
      </KunButton>
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
            <div class="text-default-500 mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
              <KunAvatar :user="r.user" size="xs" />
              <span>
                由 <span class="text-foreground font-medium">{{
                  r.user?.name ?? '未知用户'
                }}</span> 发布于
                {{
                  formatDate(r.created, { isShowYear: true, isPrecise: true })
                }}
              </span>
              <!-- "编辑时间" chip — only show when update_time > created (i.e.
                   the row has been edited at least once). Backend stamps
                   UpdateTime = time.Now() on every UpdateResource and leaves
                   it = created on insert, so the !== check is reliable. -->
              <KunChip
                v-if="hasBeenEdited(r)"
                color="default"
                variant="flat"
                size="xs"
              >
                <KunIcon name="lucide:pencil-line" class="size-3" />
                {{ formatDistanceToNow(r.update_time!) }}更新
              </KunChip>
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
            <KunButton
              :variant="r.is_liked ? 'flat' : 'light'"
              color="danger"
              size="xs"
              rounded="full"
              :aria-label="r.is_liked ? '取消点赞' : '点赞'"
              @click="toggleLike(r)"
            >
              <KunIcon
                name="lucide:heart"
                :class="cn('size-4', r.is_liked && 'fill-current')"
              />
              {{ r.like_count }}
            </KunButton>
            <span class="flex items-center gap-1.5">
              <KunIcon name="lucide:download" class="size-4" />
              {{ r.download }}
            </span>
            <!-- Owner / moderator: edit + delete. Backend re-checks the
                 same predicate so a hostile client can't bypass by forging
                 visibility. -->
            <KunButton
              v-if="canEdit(r)"
              variant="light"
              color="default"
              size="xs"
              rounded="full"
              aria-label="编辑资源"
              @click="askEdit(r)"
            >
              <KunIcon name="lucide:pencil" class="size-4" />
              编辑
            </KunButton>
            <KunButton
              v-if="canDelete(r)"
              variant="light"
              color="danger"
              size="xs"
              rounded="full"
              aria-label="删除资源"
              @click="askDelete(r)"
            >
              <KunIcon name="lucide:trash-2" class="size-4" />
              删除
            </KunButton>
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
              <KunButton
                variant="light"
                color="success"
                size="xs"
                is-icon-only
                class-name="shrink-0"
                aria-label="复制下载链接"
                title="复制下载链接"
                @click="useKunCopy(lnk)"
              >
                <KunIcon name="lucide:copy" class="size-4" />
              </KunButton>
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

    <KunModal v-model="publishOpen" inner-class-name="max-w-3xl">
      <ResourcePublish
        v-if="publishOpen"
        :patch-id="galgameId"
        @close="publishOpen = false"
        @success="handlePublishSuccess"
      />
    </KunModal>

    <KunModal v-model="editOpen" inner-class-name="max-w-3xl">
      <ResourcePublish
        v-if="editOpen && editingResource"
        :patch-id="galgameId"
        :resource="editingResource"
        @close="editOpen = false"
        @success="handleEditSuccess"
      />
    </KunModal>

    <KunModal v-model="deleteOpen" inner-class-name="max-w-md">
      <div class="space-y-4 py-2">
        <h3 class="text-lg font-bold">删除补丁资源？</h3>
        <p class="text-default-600 text-sm">
          此操作不可撤销。资源记录会从数据库移除，对应的 S3 文件也会被删除。
        </p>
        <p
          v-if="pendingDelete?.name"
          class="text-default-500 text-sm"
        >
          <span class="text-default-400">资源名称：</span>
          <strong class="text-foreground">{{ pendingDelete.name }}</strong>
        </p>
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            color="default"
            :disabled="deleting"
            @click="deleteOpen = false"
          >
            取消
          </KunButton>
          <KunButton
            color="danger"
            :loading="deleting"
            :disabled="deleting"
            @click="confirmDelete"
          >
            <KunIcon v-if="!deleting" name="lucide:trash-2" class="size-4" />
            确认删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
