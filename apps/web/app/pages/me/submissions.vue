<script setup lang="ts">
import type { KunUIColor } from '@kungal/ui-core'
import { kunMoyuMoe } from '~/config/moyu-moe'

useKunDisableSeo('我的提交')

const route = useRoute()
const api = useApi()

interface MineItem {
  work_id: number
  display_name: string
  claim_state: string
  product_work_id: number | null
  last_reason: string | null
  first_acted_at: string
}
interface MineResp {
  items: MineItem[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<MineResp>(
  'me-submissions',
  async () => {
    const res = await api.get<MineResp>('/galgame/mine?limit=50')
    if (res.code !== 0) return { items: [], total: 0 }
    return {
      items: res.data?.items ?? [],
      total: res.data?.total ?? 0
    }
  },
  { default: () => ({ items: [], total: 0 }) }
)

// This site's page id is the catalog work id, so every call of ours takes
// work_id. product_work_id is the FORUM's number for the same game and is the
// only thing a link to kungal may use.
const patchID = (m: MineItem): number => m.work_id
const forumGID = (m: MineItem): number => m.product_work_id ?? m.work_id

const displayName = (m: MineItem): string => m.display_name || `#${patchID(m)}`

const STATE_LABEL: Record<string, { text: string; color: KunUIColor }> = {
  none: { text: '未认领', color: 'default' },
  draft: { text: '草稿', color: 'default' },
  pending: { text: '审核中', color: 'warning' },
  live: { text: '已发布', color: 'success' },
  declined: { text: '已拒绝', color: 'danger' },
  hidden: { text: '已封禁', color: 'danger' }
}

const stateLabel = (s: string): { text: string; color: KunUIColor } =>
  STATE_LABEL[s] ?? { text: s, color: 'default' }

const withdrawing = ref<number | null>(null)
const handleWithdraw = async (m: MineItem) => {
  const ok = await useKunAlert({
    title: '撤回提交',
    message: `确定要撤回《${displayName(m)}》的提交吗？该条目会一并删除，无法恢复，之后任何人都可以重新提交该作品。`
  })
  if (!ok) return
  withdrawing.value = patchID(m)
  try {
    const res = await api.delete(`/galgame/${patchID(m)}`)
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

const handleEdit = async (m: MineItem) => {
  await navigateTo(`${kunMoyuMoe.domain.kungal}/galgame/${forumGID(m)}`, {
    external: true
  })
}
</script>

<template>
  <AuthRequired>
    <div class="container mx-auto my-4">
    <KunHeader
      name="我的提交"
      description="查看您提交到 Galgame 资料库的作品的审核进度"
    />

    <KunLoading v-if="pending" class-name="mt-6" description="加载中..." />

    <KunNull
      v-else-if="!data?.items?.length"
      class-name="mt-6"
      description="您还没有提交过任何作品。回到「发布 Galgame」即可开始。"
    />

    <div v-else class="mt-6 space-y-3">
      <KunCard v-for="m in data.items" :key="m.work_id" :bordered="true">
        <div class="space-y-3 p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold">{{ displayName(m) }}</h3>
                <KunChip :color="stateLabel(m.claim_state).color" size="sm">
                  {{ stateLabel(m.claim_state).text }}
                </KunChip>
              </div>
              <p class="text-default-500 mt-1 text-xs">
                提交于
                {{ formatDate(m.first_acted_at, { isPrecise: true, isShowYear: true }) }}
              </p>
            </div>
          </div>

          <div
            v-if="m.claim_state === 'declined' && m.last_reason"
            class="border-danger/30 bg-danger/10 rounded-lg border p-3 text-sm"
          >
            <p class="text-danger font-semibold">被拒原因</p>
            <p class="text-default-700 mt-1">{{ m.last_reason }}</p>
          </div>

          <div class="flex flex-wrap justify-end gap-2">
            <KunButton
              variant="bordered"
              color="danger"
              size="sm"
              :loading="withdrawing === patchID(m)"
              :disabled="withdrawing !== null"
              @click="handleWithdraw(m)"
            >
              撤回
            </KunButton>
            <KunButton
              v-if="m.claim_state === 'declined'"
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
  </AuthRequired>
</template>
