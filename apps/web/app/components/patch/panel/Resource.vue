<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()
const { open: openReport } = useReportModal()

const galgameId = computed(() => Number(route.params.id))

const { data: resources, pending } = await useAsyncData<PatchResource[]>(
  () => `patch-resource-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchResource[]>(
      `/patch/${galgameId.value}/resource`
    )
    return res.code === 0 ? res.data : []
  },
  { default: () => [], deep: true }
)

const publishOpen = ref(false)
const handlePublishClick = () => {
  if (!requireLogin()) return
  publishOpen.value = true
}
const handlePublishSuccess = (created: PatchResource) => {
  if (resources.value) {
    resources.value = [created, ...resources.value]
  }
}

const canEdit = (r: PatchResource) =>
  userStore.isModerator || r.user_id === userStore.user.id
const canDelete = (r: PatchResource) =>
  userStore.isModerator || r.user_id === userStore.user.id
const canManage = (r: PatchResource) =>
  userStore.isModerator || r.user_id === userStore.user.id

const isDisabled = (r: PatchResource) => (r.status ?? 0) !== 0

const togglingDisable = ref<number | null>(null)
const toggleDisable = async (r: PatchResource) => {
  togglingDisable.value = r.id
  try {
    const res = await api.put<{ status: number }>(
      `/patch/resource/${r.id}/disable`
    )
    if (res.code === 0) {
      r.status = res.data.status
      useKunMessage(
        isDisabled(r) ? '已禁用该资源下载' : '已恢复该资源下载',
        'success'
      )
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    togglingDisable.value = null
  }
}

const editOpen = ref(false)
const editingResource = ref<PatchResource | null>(null)
const askEdit = (r: PatchResource) => {
  editingResource.value = r
  editOpen.value = true
}
const handleEditSuccess = (updated: PatchResource) => {
  if (!resources.value) return
  resources.value = resources.value.map((r) =>
    r.id === updated.id ? updated : r
  )
}

const hasBeenEdited = (r: PatchResource) => {
  if (!r.update_time) return false
  const u = new Date(r.update_time as string | Date).getTime()
  const c = new Date(r.created).getTime()
  return Number.isFinite(u) && Number.isFinite(c) && u - c > 1000
}

const deleteOpen = ref(false)
const deleting = ref(false)
const pendingDelete = ref<PatchResource | null>(null)
const deleteReason = ref('')
const isForeignDelete = computed(
  () => !!pendingDelete.value && pendingDelete.value.user_id !== userStore.user.id
)

const askDelete = (r: PatchResource) => {
  pendingDelete.value = r
  deleteReason.value = ''
  deleteOpen.value = true
}

const confirmDelete = async () => {
  const r = pendingDelete.value
  if (!r) return
  deleting.value = true
  try {
    const res = await api.delete(
      `/patch/resource/${r.id}`,
      isForeignDelete.value ? { reason: deleteReason.value.trim() } : undefined
    )
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

type SortField = 'update_time' | 'created' | 'download'
const sortField = ref<SortField>('update_time')
const sortDesc = ref(true)

const sortOptions = [
  { value: 'update_time', label: '更改时间' },
  { value: 'created', label: '发布时间' },
  { value: 'download', label: '下载数' }
]

const timeOf = (r: PatchResource, f: 'update_time' | 'created') => {
  const v = f === 'update_time' ? (r.update_time ?? r.created) : r.created
  return new Date(v as string).getTime()
}

const sortedResources = computed(() => {
  const list = [...(resources.value ?? [])]
  const f = sortField.value
  list.sort((a, b) => {
    const cmp =
      f === 'download'
        ? (a[f] ?? 0) - (b[f] ?? 0)
        : timeOf(a, f) - timeOf(b, f)
    return sortDesc.value ? -cmp : cmp
  })
  return list
})

const onResourceFavoriteChange = async (r: PatchResource, active: boolean) => {
  if (!requireLogin()) {
    r.is_favorite = !active
    return
  }
  const res = await api.put<{ favorited: boolean }>(
    `/patch/resource/${r.id}/favorite`
  )
  if (res.code === 0) {
    r.is_favorite = res.data.favorited
    useKunMessage(
      res.data.favorited
        ? '已收藏此资源，下载链接或文件更新时会通知你'
        : '已取消收藏',
      'success'
    )
  } else {
    r.is_favorite = !active
    useKunMessage(res.message || '操作失败', 'error')
  }
}

interface ResourceMenuItem {
  key: 'edit' | 'delete' | 'disable' | 'history' | 'share' | 'report'
  label: string
  icon: string
  color?:
    | 'default'
    | 'primary'
    | 'secondary'
    | 'success'
    | 'warning'
    | 'danger'
    | 'info'
  disabled?: boolean
}

const menuItems = (r: PatchResource): ResourceMenuItem[] => {
  const items: ResourceMenuItem[] = []
  if (canEdit(r)) {
    items.push({ key: 'edit', label: '编辑', icon: 'lucide:pencil' })
  }
  if (canManage(r)) {
    items.push({
      key: 'disable',
      label: isDisabled(r) ? '恢复下载' : '禁用下载',
      icon: isDisabled(r) ? 'lucide:download' : 'lucide:ban',
      color: isDisabled(r) ? 'success' : 'warning',
      disabled: togglingDisable.value === r.id
    })
  }
  items.push({ key: 'history', label: '更改历史', icon: 'lucide:history' })
  items.push({ key: 'share', label: '分享', icon: 'lucide:share-2' })
  if (r.user_id !== userStore.user.id) {
    items.push({ key: 'report', label: '举报', icon: 'lucide:flag', color: 'danger' })
  }
  if (canDelete(r)) {
    items.push({
      key: 'delete',
      label: '删除',
      icon: 'lucide:trash-2',
      color: 'danger'
    })
  }
  return items
}

const onMenuSelect = (r: PatchResource, item: { key: string }) => {
  switch (item.key) {
    case 'edit':
      askEdit(r)
      break
    case 'delete':
      askDelete(r)
      break
    case 'disable':
      toggleDisable(r)
      break
    case 'history':
      openHistory(r)
      break
    case 'share':
      shareResource(r)
      break
    case 'report':
      reportResource(r)
      break
  }
}

const reportResource = (r: PatchResource) => {
  if (!requireLogin()) return
  openReport({
    subjectKind: 'patch_resource',
    subjectId: r.id,
    subjectUrl: `${kunMoyuMoe.domain.main}/resource/${r.id}`,
    snapshot: r.name
  })
}

const patch = inject<Ref<PatchHeader | null>>('patch')
const galgameName = computed(() =>
  patch?.value ? getPreferredLanguageText(patch.value.name) : ''
)
const shareResource = (r: PatchResource) => {
  const url = `${location.origin}/resource/${r.id}`
  const text = `${galgameName.value}${r.name || '补丁资源'}资源下载: ${url}`
  navigator.clipboard
    .writeText(text)
    .then(() => useKunMessage('链接复制成功', 'success'))
    .catch(() => useKunMessage('复制失败,请手动复制', 'error'))
}

interface ResourceFieldChange {
  field: string
  label: string
  before: string
  after: string
}
interface ResourceRevisionItem {
  id: number
  action: string
  reason: string
  actor_role: number
  created_at: string
  changes: ResourceFieldChange[]
}
const ACTOR_ROLE_LABEL: Record<number, string> = {
  0: '未知',
  1: '普通用户',
  2: '版主',
  3: '管理员'
}
const histOpen = ref(false)
const histResource = ref<PatchResource | null>(null)
const histLoading = ref(false)
const histItems = ref<ResourceRevisionItem[]>([])
const histTotal = ref(0)
const histPage = ref(1)
const histLimit = 20
const histTotalPages = computed(() =>
  Math.max(1, Math.ceil(histTotal.value / histLimit))
)

const loadHistory = async () => {
  if (!histResource.value) return
  histLoading.value = true
  try {
    const res = await api.get<{
      items: ResourceRevisionItem[]
      total: number
    }>(
      `/patch/resource/${histResource.value.id}/revisions?page=${histPage.value}&limit=${histLimit}`
    )
    if (res.code === 0) {
      histItems.value = res.data?.items ?? []
      histTotal.value = res.data?.total ?? 0
    } else {
      useKunMessage(res.message || '加载更改历史失败', 'error')
    }
  } finally {
    histLoading.value = false
  }
}

const openHistory = async (r: PatchResource) => {
  histResource.value = r
  histPage.value = 1
  histItems.value = []
  histTotal.value = 0
  histOpen.value = true
  await loadHistory()
}

watch(histPage, loadHistory)
</script>

<template>
  <div class="space-y-4">
    <div class="flex justify-end">
      <KunButton color="primary" @click="handlePublishClick">
        <KunIcon name="lucide:plus" class="size-4" />
        发布资源
      </KunButton>
    </div>

    <KunAdAIEroBanner />

    <div
      v-if="!pending && resources && resources.length"
      class="border-default/20 bg-content1 shadow-kun-sm flex flex-wrap items-center gap-2 rounded-2xl border p-3"
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
        class="border-default/20 bg-content1 shadow-kun-sm space-y-4 rounded-2xl border p-5 transition-colors"
      >
        <div class="flex items-start gap-2">
          <div class="min-w-0 flex-1">
            <h3 class="text-lg font-semibold line-clamp-2">
              {{ r.name || '补丁资源' }}
            </h3>
            <div class="text-default-500 mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
              <KunAvatar :user="r.user" size="sm" />
              <span>
                由
                <NuxtLink
                  v-if="r.user?.id"
                  :to="`/user/${r.user.id}/resource`"
                  class="text-foreground hover:text-primary font-medium hover:underline"
                >
                  {{ r.user.name }}
                </NuxtLink>
                <span v-else class="text-foreground font-medium">未知用户</span>
                发布于
                {{
                  formatDate(r.created, { isShowYear: true, isPrecise: true })
                }}
              </span>
              <KunChip color="warning" size="xs" variant="flat">
                <KunIcon name="lucide:database" class="size-3" />
                {{ r.size }}
              </KunChip>
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
          <KunDropdown
            :items="menuItems(r)"
            position="bottom-end"
            @select="(item) => onMenuSelect(r, item)"
          >
            <template #trigger>
              <KunButton
                is-icon-only
                variant="light"
                color="default"
                size="sm"
                rounded="full"
                class-name="shrink-0"
                aria-label="更多操作"
              >
                <KunIcon name="lucide:ellipsis-vertical" class="size-4" />
              </KunButton>
            </template>
          </KunDropdown>
        </div>

        <div
          v-if="isDisabled(r)"
          class="border-danger/30 bg-danger/10 text-danger-700 flex items-center gap-2 rounded-xl border p-3 text-sm"
        >
          <KunIcon name="lucide:shield-alert" class="size-4 shrink-0" />
          <span>
            该资源已被禁用下载（可能存在安全风险，或应发布者 / 版主要求下架），暂时无法获取下载链接。
          </span>
        </div>

        <KunPatchAttribute
          :types="r.type"
          :languages="r.language"
          :platforms="r.platform"
          :model-name="r.model_name"
          :storage="r.storage"
          size="sm"
        />

        <ResourceNote v-if="r.note_html" :html="r.note_html" :max-height="100" />

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
          class="border-default/15 flex flex-wrap items-center justify-between gap-2 border-t pt-3"
        >
          <div class="flex min-w-0 flex-col gap-1.5">
            <div class="text-default-500 flex items-center gap-4 text-sm">
              <KunReaction
                v-model="r.is_favorite"
                icon="lucide:star"
                color="warning"
                size="sm"
                @change="(active) => onResourceFavoriteChange(r, active)"
              >
                {{ r.is_favorite ? '已收藏' : '收藏资源' }}
              </KunReaction>
              <span class="flex items-center gap-1.5">
                <KunIcon name="lucide:download" class="size-4" />
                {{ r.download }}
              </span>
            </div>
            <p
              :class="
                cn(
                  'flex items-center gap-1 text-xs',
                  r.is_favorite ? 'text-warning' : 'text-default-400'
                )
              "
            >
              <KunIcon name="lucide:bell" class="size-3 shrink-0" />
              {{
                r.is_favorite
                  ? '已收藏此资源，下载链接或文件更新时会通知你'
                  : '收藏此资源，下载链接或文件更新时通知你'
              }}
            </p>
          </div>
          <div class="flex shrink-0 flex-wrap items-center gap-2">
            <KunChip
              v-if="isDisabled(r)"
              color="danger"
              variant="flat"
              size="sm"
            >
              <KunIcon name="lucide:ban" class="size-3.5" />
              已禁用下载
            </KunChip>
            <KunButton
              color="primary"
              size="sm"
              rounded="full"
              :href="`/resource/${r.id}`"
            >
              <KunIcon name="lucide:arrow-up-right" class="size-4" />
              查看资源详情
            </KunButton>
          </div>
        </div>

      </div>
    </div>
    <KunNull v-else description="该 Galgame 暂无补丁资源" />

    <KunModal
      v-model="publishOpen"
      inner-class-name="max-w-3xl"
      :is-dismissable="false"
      aria-label="发布补丁资源"
    >
      <ResourcePublish
        v-if="publishOpen"
        :patch-id="galgameId"
        @close="publishOpen = false"
        @success="handlePublishSuccess"
      />
    </KunModal>

    <KunModal
      v-model="editOpen"
      inner-class-name="max-w-3xl"
      :is-dismissable="false"
      aria-label="编辑补丁资源"
    >
      <ResourcePublish
        v-if="editOpen && editingResource"
        :patch-id="galgameId"
        :resource="editingResource"
        @close="editOpen = false"
        @success="handleEditSuccess"
      />
    </KunModal>

    <KunModal
      v-model="deleteOpen"
      inner-class-name="max-w-md"
      aria-label="删除补丁资源"
    >
      <div class="space-y-4 py-2">
        <h3 class="text-lg font-bold">删除补丁资源？</h3>
        <p class="text-default-600 text-sm">
          此操作不可撤销。资源记录会从数据库移除，对应的下载文件也会一并删除。
        </p>
        <p
          v-if="pendingDelete?.name"
          class="text-default-500 text-sm"
        >
          <span class="text-default-400">资源名称：</span>
          <strong class="text-foreground">{{ pendingDelete.name }}</strong>
        </p>
        <div v-if="isForeignDelete" class="space-y-1">
          <label class="text-default-600 text-sm">
            删除原因（可选，会通知作者并记入管理日志）
          </label>
          <KunInput
            v-model="deleteReason"
            placeholder="例如：转载自付费站 / 违规内容 / 重复发布"
          />
        </div>
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

    <KunModal
      v-model="histOpen"
      inner-class-name="max-w-xl"
      aria-label="资源更改历史"
    >
      <div class="space-y-4 py-2">
        <div class="flex items-center gap-2">
          <KunIcon name="lucide:history" class="text-primary size-5" />
          <h3 class="text-lg font-bold">资源更改历史</h3>
        </div>
        <p v-if="histResource?.name" class="text-default-500 text-sm">
          {{ histResource.name }}
        </p>

        <KunLoading v-if="histLoading" description="正在加载更改历史..." />
        <div v-else-if="histItems.length" class="space-y-3">
          <div
            v-for="rev in histItems"
            :key="rev.id"
            class="border-default/20 bg-default-50 space-y-3 rounded-xl border p-3"
          >
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <KunChip color="primary" variant="flat" size="xs">
                <KunIcon name="lucide:pencil-line" class="size-3" />
                编辑
              </KunChip>
              <span class="text-default-500">
                {{ formatDate(rev.created_at, { isShowYear: true, isPrecise: true }) }}
              </span>
              <KunChip color="default" variant="flat" size="xs">
                {{ ACTOR_ROLE_LABEL[rev.actor_role] ?? '未知' }}
              </KunChip>
              <span v-if="rev.reason" class="text-default-400">
                原因：{{ rev.reason }}
              </span>
            </div>

            <div class="space-y-2">
              <div v-for="(c, i) in rev.changes" :key="i">
                <div class="text-default-500 mb-1 text-xs font-medium">
                  {{ c.label }}
                </div>
                <div class="flex items-stretch gap-2">
                  <div
                    class="border-danger/30 bg-danger/5 text-danger-700 min-w-0 flex-1 rounded-lg border px-2 py-1 text-sm break-words"
                  >
                    <span class="text-default-400 block text-[10px]">改动前</span>
                    {{ c.before || '(空)' }}
                  </div>
                  <KunIcon
                    name="lucide:arrow-right"
                    class="text-default-400 size-4 shrink-0 self-center"
                  />
                  <div
                    class="border-success/30 bg-success/5 text-success-700 min-w-0 flex-1 rounded-lg border px-2 py-1 text-sm break-words"
                  >
                    <span class="text-default-400 block text-[10px]">改动后</span>
                    {{ c.after || '(空)' }}
                  </div>
                </div>
              </div>
              <p v-if="!rev.changes.length" class="text-default-400 text-sm">
                无字段变化
              </p>
            </div>
          </div>

          <div v-if="histTotalPages > 1" class="flex justify-center pt-1">
            <KunPagination
              v-model:current-page="histPage"
              :total-page="histTotalPages"
            />
          </div>
        </div>
        <KunNull v-else description="暂无更改历史" />

        <div class="flex justify-end">
          <KunButton variant="light" color="default" @click="histOpen = false">
            关闭
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
