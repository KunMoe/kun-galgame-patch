<script setup lang="ts">
// Trust & Safety moderation inbox. Proxies the infra trust admin API through
// moyu's BFF (/admin/trust/*, moderatorAuth, site forced to moyu). A moderator
// lists review items, opens the evidence (reports + snapshots), claims, then
// dismisses or applies an enforcement action — which fires the signed callback
// back to moyu (hide/remove/restore) or the IdP.
import {
  TRUST_REVIEW_STATUS,
  TRUST_REVIEW_SOURCE,
  TRUST_ACTIONS,
  TRUST_SUBJECT_KIND,
  trustSubjectHref
} from '~/constants/trust'

useKunDisableSeo('内容审核')

const api = useApi()

const statusOptions = [
  { value: 0, label: '待处理' },
  { value: 1, label: '处理中' },
  { value: 2, label: '已处置' },
  { value: 3, label: '已驳回' },
  { value: -1, label: '全部' }
]
const filterStatus = ref(0)
const page = ref(1)
const limit = 30

const { data, pending, refresh } = await useAsyncData<ReviewItemPage>(
  'admin-moderation',
  async () => {
    const res = await api.get<ReviewItemPage>(
      `/admin/trust/review-items?status=${filterStatus.value}&source=-1&page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch(filterStatus, () => {
  page.value = 1
  refresh()
})
watch(page, () => refresh())

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
const kindLabel = (k: string) => TRUST_SUBJECT_KIND[k] ?? k

// ── Detail modal ──
const isDetailOpen = ref(false)
const detail = ref<ReviewItemDetail | null>(null)
const detailLoading = ref(false)

const decision = ref<'actioned' | 'dismissed'>('actioned')
const action = ref(1)
const reasonCode = ref('')
const statement = ref('')
const isWorking = ref(false)

const openDetail = async (id: number) => {
  isDetailOpen.value = true
  detailLoading.value = true
  detail.value = null
  decision.value = 'actioned'
  action.value = 1
  reasonCode.value = ''
  statement.value = ''
  const res = await api.get<ReviewItemDetail>(`/admin/trust/review-items/${id}`)
  detail.value = res.code === 0 ? res.data : null
  detailLoading.value = false
}

// Only pending (0) / claimed (1) items are still actionable.
const isActionable = computed(() => {
  const s = detail.value?.item.status
  return s === 0 || s === 1
})

// Prefer the reporter-carried subject_url (works for every kind, incl.
// patch_comment which has no page of its own), else reconstruct from kind+id.
const subjectHref = computed(() => {
  if (!detail.value) return undefined
  const fromReport = detail.value.reports.find((r) => r.subject_url)?.subject_url
  return (
    fromReport ||
    trustSubjectHref(
      detail.value.item.subject_kind,
      detail.value.item.subject_id
    )
  )
})

const claim = async () => {
  if (!detail.value) return
  isWorking.value = true
  const res = await api.post(
    `/admin/trust/review-items/${detail.value.item.id}/claim`
  )
  isWorking.value = false
  if (res.code === 0) {
    useKunMessage('已认领', 'success')
    await openDetail(detail.value.item.id)
    refresh()
  } else {
    useKunMessage(res.message || '认领失败', 'error')
  }
}

const decide = async () => {
  if (!detail.value) return
  if (decision.value === 'actioned' && !reasonCode.value.trim()) {
    useKunMessage('请填写处置理由代码（reason_code）', 'warn')
    return
  }
  isWorking.value = true
  const body: Record<string, unknown> = { decision: decision.value }
  if (decision.value === 'actioned') {
    body.action = action.value
    body.reason_code = reasonCode.value.trim()
    if (statement.value.trim()) body.statement = statement.value.trim()
  }
  const res = await api.post(
    `/admin/trust/review-items/${detail.value.item.id}/decide`,
    body
  )
  isWorking.value = false
  if (res.code === 0) {
    useKunMessage('已处置', 'success')
    isDetailOpen.value = false
    refresh()
  } else {
    useKunMessage(res.message || '处置失败', 'error')
  }
}

const actionOptions = TRUST_ACTIONS.map((a) => ({
  value: a.value as number,
  label: a.label
}))
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-2xl font-bold">内容审核</h1>
      <p class="text-default-500 text-sm">
        来自 Trust &amp; Safety 平台的统一审核队列，按优先级排序。
      </p>
    </div>

    <div class="max-w-xs">
      <KunSelect v-model="filterStatus" :options="statusOptions" label="状态" />
    </div>

    <KunNull
      v-if="!pending && !data?.items.length"
      description="暂无审核条目"
    />

    <div v-else class="space-y-2">
      <button
        v-for="item in data?.items ?? []"
        :key="item.id"
        class="hover:bg-default-100 border-default-200 w-full rounded-lg border p-3 text-left transition-colors"
        @click="openDetail(item.id)"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="flex flex-wrap items-center gap-2">
            <KunChip color="secondary">{{ kindLabel(item.subject_kind) }}</KunChip>
            <span class="text-default-500 text-sm">#{{ item.subject_id }}</span>
            <KunChip :color="TRUST_REVIEW_STATUS[item.status]?.color ?? 'default'">
              {{ TRUST_REVIEW_STATUS[item.status]?.label ?? item.status }}
            </KunChip>
          </div>
          <span class="text-default-400 shrink-0 text-xs">
            优先级 {{ item.priority.toFixed(1) }}
          </span>
        </div>
        <div
          class="text-default-400 mt-1 flex flex-wrap items-center gap-x-3 text-xs"
        >
          <span>来源：{{ TRUST_REVIEW_SOURCE[item.source] ?? item.source }}</span>
          <span v-if="item.report_weight_sum">
            举报权重 {{ item.report_weight_sum.toFixed(1) }}
          </span>
          <span>{{ formatDate(item.created_at, { isShowYear: true, isPrecise: true }) }}</span>
        </div>
      </button>
    </div>

    <KunPagination
      v-if="(data?.total ?? 0) > limit"
      v-model:current-page="page"
      :total-page="totalPages"
      :is-loading="pending"
    />

    <KunModal v-model="isDetailOpen" inner-class-name="max-w-2xl w-[94vw]">
      <KunLoading v-if="detailLoading" />
      <div
        v-else-if="detail"
        class="max-h-[80dvh] space-y-4 overflow-y-auto p-1"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-lg font-bold">审核详情</span>
          <KunChip color="secondary">
            {{ kindLabel(detail.item.subject_kind) }}
          </KunChip>
          <NuxtLink
            v-if="subjectHref"
            :to="subjectHref"
            target="_blank"
            class="text-primary text-sm"
          >
            查看内容 #{{ detail.item.subject_id }}
          </NuxtLink>
          <span v-else class="text-default-500 text-sm">
            #{{ detail.item.subject_id }}
          </span>
        </div>

        <!-- Reports (evidence) -->
        <div class="space-y-2">
          <span class="text-default-600 text-sm font-medium">
            举报记录（{{ detail.reports.length }}）
          </span>
          <div
            v-for="r in detail.reports"
            :key="r.id"
            class="bg-default-100 space-y-1 rounded-lg p-2 text-sm"
          >
            <div class="text-default-400 flex flex-wrap gap-x-3 text-xs">
              <span>举报人 #{{ r.reporter_id }}</span>
              <span>理由 #{{ r.reason_id }}</span>
              <span>权重 {{ r.weight.toFixed(1) }}</span>
              <span>{{ formatDate(r.created_at, { isShowYear: true, isPrecise: true }) }}</span>
            </div>
            <p v-if="r.note" class="text-default-700">{{ r.note }}</p>
            <pre
              v-if="r.subject_snapshot"
              class="text-default-500 border-default-200 max-h-32 overflow-y-auto rounded border p-2 text-xs whitespace-pre-wrap"
            >{{ r.subject_snapshot }}</pre>
          </div>
        </div>

        <!-- Decision -->
        <template v-if="isActionable">
          <div class="border-default-200 border-t" />
          <div class="space-y-3">
            <div class="flex items-center gap-2">
              <KunButton
                v-if="detail.item.status === 0"
                variant="flat"
                color="primary"
                :loading="isWorking"
                @click="claim"
              >
                认领
              </KunButton>
              <span v-else class="text-default-500 text-sm">
                处理中（认领人 #{{ detail.item.claimed_by }}）
              </span>
            </div>

            <div class="flex gap-2">
              <KunButton
                :variant="decision === 'actioned' ? 'flat' : 'light'"
                color="danger"
                size="sm"
                @click="decision = 'actioned'"
              >
                处置
              </KunButton>
              <KunButton
                :variant="decision === 'dismissed' ? 'flat' : 'light'"
                color="default"
                size="sm"
                @click="decision = 'dismissed'"
              >
                驳回
              </KunButton>
            </div>

            <template v-if="decision === 'actioned'">
              <KunSelect
                v-model="action"
                :options="actionOptions"
                label="处置动作"
              />
              <KunInput
                v-model="reasonCode"
                label="处置理由代码 (reason_code)"
                placeholder="例如 spam / abuse / copyright"
              />
              <KunTextarea
                name="statement"
                v-model="statement"
                placeholder="面向用户的处置说明（可选）"
                :rows="3"
              />
            </template>

            <div class="flex justify-end">
              <KunButton color="danger" :loading="isWorking" @click="decide">
                确认{{ decision === 'actioned' ? '处置' : '驳回' }}
              </KunButton>
            </div>
          </div>
        </template>
        <p v-else class="text-default-500 text-sm">
          该条目已终结（{{ TRUST_REVIEW_STATUS[detail.item.status]?.label }}）。
        </p>
      </div>
    </KunModal>
  </div>
</template>
