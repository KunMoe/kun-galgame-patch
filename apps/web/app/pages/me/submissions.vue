<script setup lang="ts">
import type { KunUIColor } from '@kun/ui/app/components/kun/ui/type'
// "My submissions" page — proxies GET /galgame/mine.
//
// Shows the caller's status ∈ {3, 4} drafts so they can:
//   - See current state (pending / declined with reason)
//   - Re-edit a declined draft (auto-flips back to status=3 on save)
//   - Withdraw a draft via DELETE /galgame/:gid
//
// See docs/galgame_wiki/07-submission.md §GET /galgame/mine.

useKunSeoMeta({
  title: '我的提交',
  description: '查看您提交到 Galgame Wiki 的作品审核进度'
})

const route = useRoute()
const userStore = useUserStore()
const api = useApi()

if (!userStore.isLoggedIn) {
  await navigateTo({ path: '/login', query: { from: route.fullPath } })
}

interface MineItem {
  id: number
  status: number
  vndb_id: string
  name_en_us: string
  name_ja_jp: string
  name_zh_cn: string
  name_zh_tw: string
  banner: string
  effective_banner_hash: string
  content_limit: string
  created: string
  updated: string
  decline_reason?: string
}
interface MineResp {
  items: MineItem[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<MineResp>(
  'me-submissions',
  async () => {
    const res = await api.get<MineResp>('/galgame/mine?status=3,4&limit=50')
    if (res.code !== 0) return { items: [], total: 0 }
    return {
      items: res.data?.items ?? [],
      total: res.data?.total ?? 0
    }
  },
  { default: () => ({ items: [], total: 0 }) }
)

const displayName = (m: MineItem): string =>
  m.name_zh_cn || m.name_zh_tw || m.name_ja_jp || m.name_en_us || `#${m.id}`

const statusLabel = (s: number): { text: string; color: KunUIColor } => {
  if (s === 3) return { text: '审核中', color: 'warning' }
  if (s === 4) return { text: '已拒绝', color: 'danger' }
  return { text: `状态 ${s}`, color: 'default' }
}

// ─── Withdraw (DELETE /galgame/:gid) ──────────────────
const withdrawing = ref<number | null>(null)
const handleWithdraw = async (m: MineItem) => {
  const ok = await useKunAlert({
    title: '撤回提交',
    message: `确定要撤回《${displayName(m)}》的提交吗？撤回后无法恢复，需要重新提交。`
  })
  if (!ok) return
  withdrawing.value = m.id
  try {
    const res = await api.delete(`/galgame/${m.id}`)
    if (res.code === 0) {
      useKunMessage('已撤回', 'success')
      await refresh()
      return
    }
    useKunMessage(res.message || '撤回失败', 'error')
  } finally {
    withdrawing.value = null
  }
}

// ─── Edit (navigate to a single-draft edit page) ──────
const handleEdit = async (m: MineItem) => {
  // Re-use the publish wizard's submit form by passing the draft id in the
  // query — but for now navigate to a dedicated edit route. The wizard does
  // not yet support pre-loading a draft, so we use rewrite.vue with a flag.
  await navigateTo(`/edit/draft?id=${m.id}`)
}
</script>

<template>
  <div class="container mx-auto my-4">
    <KunHeader
      name="我的提交"
      description="查看您提交到 Galgame Wiki 的作品的审核进度"
    />

    <KunLoading v-if="pending" class-name="mt-6" description="加载中..." />

    <KunNull
      v-else-if="!data?.items?.length"
      class-name="mt-6"
      description="您还没有提交过任何作品。回到「发布 Galgame」即可开始。"
    />

    <div v-else class="mt-6 space-y-3">
      <KunCard v-for="m in data.items" :key="m.id" :bordered="true">
        <div class="space-y-3 p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold">{{ displayName(m) }}</h3>
                <KunChip :color="statusLabel(m.status).color" size="sm">
                  {{ statusLabel(m.status).text }}
                </KunChip>
              </div>
              <p class="text-default-500 mt-1 text-xs">
                {{ m.vndb_id || '无 VNDB ID' }} · 提交于
                {{ formatDate(m.created, { isPrecise: true, isShowYear: true }) }}
              </p>
            </div>
          </div>

          <!-- Declined: surface the admin reason inline -->
          <div
            v-if="m.status === 4 && m.decline_reason"
            class="border-danger/30 bg-danger/10 rounded-lg border p-3 text-sm"
          >
            <p class="text-danger font-semibold">被拒原因</p>
            <p class="text-default-700 mt-1">{{ m.decline_reason }}</p>
          </div>

          <div class="flex flex-wrap justify-end gap-2">
            <KunButton
              variant="bordered"
              color="danger"
              size="sm"
              :loading="withdrawing === m.id"
              :disabled="withdrawing !== null"
              @click="handleWithdraw(m)"
            >
              撤回
            </KunButton>
            <KunButton
              v-if="m.status === 4"
              color="primary"
              size="sm"
              @click="handleEdit(m)"
            >
              重新编辑并提交
            </KunButton>
            <KunButton v-else variant="light" color="primary" size="sm" @click="handleEdit(m)">
              编辑
            </KunButton>
          </div>
        </div>
      </KunCard>
    </div>
  </div>
</template>
