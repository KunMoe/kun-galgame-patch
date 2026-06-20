<script setup lang="ts">
// 创作者申请 modal. Rendered as a SIBLING of the avatar popover in UserDropdown
// (the popover closes imperatively, so a sibling modal isn't torn down with it —
// same pattern as 萌萌点记录). Replaces the old buried settings card: it surfaces
// the creator benefits + live eligibility the moment it opens and lets an
// eligible user apply in one click. Eligibility is computed by the moyu backend
// (满足任一即可): 已发布补丁资源 ≥ 3 · 萌萌点 ≥ 2000 ·
// 已被合并的 Galgame 信息更新请求 ≥ 5.
// The application, admin review and role grant all live in OAuth; the granted
// `creator` role lets the user publish Galgame directly (incl. VNDB-less works).
const open = defineModel<boolean>({ required: true })

const api = useApi()
const userStore = useUserStore()

interface CreatorEligibility {
  eligible: boolean
  resources: number
  moemoepoint: number
  merged_prs: number
  need_resources: number
  need_moemoepoint: number
  need_merged_prs: number
}
interface CreatorApplicationInfo {
  id: number
  status: string
  decline_reason: string
}
interface CreatorStatus {
  eligibility: CreatorEligibility
  application: CreatorApplicationInfo | null
}

const status = ref<CreatorStatus | null>(null)
const message = ref('')
const loading = ref(false)
const failed = ref(false)
const submitting = ref(false)

const load = async () => {
  loading.value = true
  failed.value = false
  const res = await api.get<CreatorStatus>('/user/creator/status')
  if (res.code === 0 && res.data) {
    status.value = res.data
  } else {
    failed.value = true
  }
  loading.value = false
}

// Re-fetch on every open so eligibility is fresh; clear any stale draft.
watch(open, (v) => {
  if (v) {
    message.value = ''
    load()
  }
})

const eligibility = computed(() => status.value?.eligibility ?? null)
const application = computed(() => status.value?.application ?? null)
// userStore.roles is moyu's client-side source of truth for the role (refreshed
// via /auth/me); the approved application covers the short window right after
// approval before that refresh lands.
const isCreator = computed(
  () =>
    (userStore.user.roles?.includes('creator') ?? false) ||
    application.value?.status === 'approved'
)
const isPending = computed(() => application.value?.status === 'pending')
const isDeclined = computed(() => application.value?.status === 'declined')
const isEligible = computed(() => !!eligibility.value?.eligible)
const canApply = computed(
  () => isEligible.value && !isPending.value && !isCreator.value
)

// Application flow as a KunSteps stepper. current is 0-based; earlier steps
// render done (check). 0 达成条件 · 1 提交申请 · 2 审核 · 3 成为创作者.
const FLOW = [
  { title: '达成条件', icon: 'lucide:target' },
  { title: '提交申请', icon: 'lucide:send' },
  { title: '管理员审核', icon: 'lucide:user-round-check' },
  { title: '成为创作者', icon: 'lucide:party-popper' }
]
const currentStep = computed(() => {
  if (isCreator.value) return 3
  if (isPending.value) return 2
  if (isEligible.value) return 1
  return 0
})

const BENEFITS = [
  '直接发布 Galgame 词条，无需排队等待审核',
  '收录 VNDB 未收录的原创 / 同人 / 独立作品',
  '提交即时生效，编辑已发布条目更自由'
]

const conditions = computed(() => {
  const e = eligibility.value
  if (!e) return []
  return [
    { label: '已发布补丁资源', cur: e.resources, need: e.need_resources },
    { label: '萌萌点', cur: e.moemoepoint, need: e.need_moemoepoint },
    {
      label: '已经被合并的 Galgame 信息更新请求',
      cur: e.merged_prs,
      need: e.need_merged_prs
    }
  ].map((c) => ({
    ...c,
    met: c.cur >= c.need,
    pct: c.need > 0 ? Math.min(100, Math.round((c.cur / c.need) * 100)) : 100
  }))
})

const handleApply = async () => {
  if (!canApply.value) return
  submitting.value = true
  const res = await api.post<CreatorApplicationInfo>('/user/creator/apply', {
    message: message.value
  })
  submitting.value = false
  if (res.code === 0) {
    useKunMessage('申请已提交，等待管理员审核', 'success')
    await load()
  } else {
    useKunMessage(res.message || '提交失败', 'error')
  }
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-xl w-full border-default-200">
    <div class="space-y-5 p-1">
      <!-- header -->
      <div class="flex items-start gap-3">
        <KunIcon
          class="text-primary mt-0.5 size-8 shrink-0"
          name="lucide:badge-check"
        />
        <div class="space-y-0.5">
          <h2 class="text-lg font-semibold">成为创作者</h2>
          <p class="text-default-500 text-sm">
            创作者是社区里值得信赖的发布者，可直接为大家收录 Galgame 词条。
          </p>
        </div>
      </div>

      <KunLoading v-if="loading" description="加载创作者状态中..." />

      <div v-else-if="failed" class="flex flex-col items-center gap-3 py-10">
        <p class="text-default-500 text-sm">加载失败</p>
        <KunButton variant="flat" size="sm" @click="load">重试</KunButton>
      </div>

      <template v-else-if="status">
        <KunSteps
          :items="FLOW"
          :current="currentStep"
          color="primary"
          size="sm"
        />

        <!-- already a creator (edge: just approved before the role cache caught up) -->
        <div
          v-if="isCreator"
          class="bg-success-50 text-success-700 flex items-center gap-2 rounded-xl p-4 text-sm"
        >
          <KunIcon class="size-5 shrink-0" name="lucide:party-popper" />
          您已是创作者，可直接发布 Galgame 词条，无需再次申请。
        </div>

        <template v-else>
          <!-- benefits -->
          <section class="space-y-2">
            <h3 class="text-default-700 text-sm font-medium">创作者特权</h3>
            <ul class="text-default-700 list-disc space-y-1 pl-5 text-sm">
              <li v-for="b in BENEFITS" :key="b">{{ b }}</li>
            </ul>
          </section>

          <KunDivider />

          <!-- conditions: any-of -->
          <section class="space-y-3">
            <div class="flex items-center justify-between">
              <h3 class="text-default-700 text-sm font-medium">申请条件</h3>
              <KunChip
                size="sm"
                variant="flat"
                :color="isEligible ? 'success' : 'default'"
              >
                满足任一即可 · {{ isEligible ? '已满足' : '未满足' }}
              </KunChip>
            </div>
            <div v-for="c in conditions" :key="c.label" class="space-y-1">
              <div class="flex items-center justify-between text-sm">
                <span class="flex items-center gap-1.5">
                  <KunIcon
                    :class="
                      c.met ? 'text-success size-4' : 'text-default-300 size-4'
                    "
                    :name="c.met ? 'lucide:circle-check' : 'lucide:circle'"
                  />
                  {{ c.label }}
                </span>
                <span
                  :class="
                    c.met ? 'text-success font-medium' : 'text-default-500'
                  "
                >
                  {{ c.cur }} / {{ c.need }}
                </span>
              </div>
              <KunProgress
                :value="c.pct"
                size="sm"
                :color="c.met ? 'success' : 'primary'"
              />
            </div>
          </section>

          <!-- current application state -->
          <div
            v-if="isPending"
            class="bg-primary-50 text-primary-700 flex items-center gap-2 rounded-xl p-3 text-sm"
          >
            <KunIcon class="size-5 shrink-0" name="lucide:clock" />
            申请审核中，管理员会尽快处理，结果将通过站内消息通知您。
          </div>
          <div
            v-else-if="isDeclined"
            class="bg-warning-50 text-warning-700 space-y-1 rounded-xl p-3 text-sm"
          >
            <div class="flex items-center gap-2 font-medium">
              <KunIcon class="size-5 shrink-0" name="lucide:circle-x" />
              上次申请未通过
            </div>
            <p v-if="application?.decline_reason" class="text-warning-600 pl-7">
              原因：{{ application.decline_reason }}
            </p>
          </div>

          <!-- optional message, only when actually applying -->
          <KunTextarea
            v-if="canApply"
            v-model="message"
            placeholder="(可选) 附言：向管理员说明你的情况"
            :rows="2"
            :maxlength="500"
          />

          <!-- CTA -->
          <div class="flex items-center justify-end gap-2 pt-1">
            <KunButton variant="light" @click="open = false">稍后再说</KunButton>
            <KunButton
              v-if="!isPending"
              color="primary"
              :disabled="!canApply"
              :loading="submitting"
              @click="handleApply"
            >
              <KunIcon v-if="canApply" class="size-4" name="lucide:send" />
              {{
                canApply ? (isDeclined ? '重新申请' : '立即申请') : '继续努力'
              }}
            </KunButton>
          </div>
        </template>
      </template>
    </div>
  </KunModal>
</template>
