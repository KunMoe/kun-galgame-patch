<script setup lang="ts">
// 创作者申请 card (extracted from pages/settings/user.vue). Eligibility is
// computed by the moyu backend (published patch resources + wiki PR stats); the
// application, review and role grant all live in OAuth. The granted `creator`
// role lets the user publish Galgame directly — incl. VNDB-less doujin/indie
// works — which the wiki enforces off the JWT role.
const api = useApi()
const userStore = useUserStore()

interface CreatorEligibility {
  eligible: boolean
  merged_prs: number
  resources: number
  need_merged_prs: number
  need_resources: number
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

const STATUS_LABEL: Record<string, string> = {
  pending: '审核中',
  approved: '已通过',
  declined: '已拒绝'
}
const STATUS_COLOR: Record<string, 'primary' | 'success' | 'warning'> = {
  pending: 'primary',
  approved: 'success',
  declined: 'warning'
}

const status = ref<CreatorStatus | null>(null)
const message = ref('')
const submitting = ref(false)
const loadError = ref(false)

// Already holds the OAuth-issued creator role → nothing left to apply for.
const isCreator = computed(() => userStore.user.roles?.includes('creator') ?? false)
const isPending = computed(() => status.value?.application?.status === 'pending')
const canApply = computed(
  () => !!status.value?.eligibility.eligible && !isPending.value && !isCreator.value
)

const load = async () => {
  const res = await api.get<CreatorStatus>('/user/creator/status')
  if (res.code === 0 && res.data) {
    status.value = res.data
    loadError.value = false
  } else {
    loadError.value = true
  }
}

const apply = async () => {
  submitting.value = true
  const res = await api.post<CreatorApplicationInfo>('/user/creator/apply', {
    message: message.value
  })
  submitting.value = false
  if (res.code === 0) {
    useKunMessage('申请已提交，等待管理员审核', 'success')
    message.value = ''
    await load()
  } else {
    useKunMessage(res.message || '提交失败', 'error')
  }
}

onMounted(load)
</script>

<template>
  <KunCard :bordered="true">
    <template #header>
      <h2 class="px-1 pt-1 text-xl font-medium">创作者申请</h2>
    </template>
    <div class="space-y-4">
      <p class="text-default-500 text-sm">
        创作者可直接发布 Galgame 词条 (含无 VNDB ID 的同人 / 独立作品)。满足以下任一条件即可申请，提交后由管理员审核。
      </p>

      <KunChip v-if="isCreator" color="success" variant="flat" size="sm">
        您已是创作者
      </KunChip>

      <p v-else-if="loadError" class="text-danger-500 text-sm">
        获取创作者状态失败，请稍后刷新重试。
      </p>

      <template v-else-if="status">
        <div class="space-y-2">
          <div class="flex items-center justify-between text-sm">
            <span>合并的 PR</span>
            <span
              :class="
                status.eligibility.merged_prs >= status.eligibility.need_merged_prs
                  ? 'text-success'
                  : 'text-default-500'
              "
            >
              {{ status.eligibility.merged_prs }} / {{ status.eligibility.need_merged_prs }}
            </span>
          </div>
          <div class="flex items-center justify-between text-sm">
            <span>已发布补丁资源</span>
            <span
              :class="
                status.eligibility.resources >= status.eligibility.need_resources
                  ? 'text-success'
                  : 'text-default-500'
              "
            >
              {{ status.eligibility.resources }} / {{ status.eligibility.need_resources }}
            </span>
          </div>
        </div>

        <div
          v-if="status.application"
          class="flex flex-wrap items-center gap-2 text-sm"
        >
          <span>当前申请：</span>
          <KunChip
            :color="STATUS_COLOR[status.application.status] || 'primary'"
            variant="flat"
            size="sm"
          >
            {{ STATUS_LABEL[status.application.status] || status.application.status }}
          </KunChip>
          <span
            v-if="status.application.status === 'declined' && status.application.decline_reason"
            class="text-default-500"
          >
            原因：{{ status.application.decline_reason }}
          </span>
        </div>

        <template v-if="!isPending">
          <KunChip
            :color="status.eligibility.eligible ? 'success' : 'warning'"
            variant="flat"
            size="sm"
          >
            {{ status.eligibility.eligible ? '符合申请条件' : '尚不符合申请条件' }}
          </KunChip>

          <KunTextarea
            v-if="status.eligibility.eligible"
            v-model="message"
            placeholder="(可选) 附言：向管理员说明你的情况"
            :rows="3"
            :maxlength="500"
          />

          <div class="flex justify-end">
            <KunButton
              color="primary"
              :loading="submitting"
              :disabled="!canApply"
              @click="apply"
            >
              {{ status.application?.status === 'declined' ? '重新申请' : '提交申请' }}
            </KunButton>
          </div>
        </template>
      </template>
    </div>
  </KunCard>
</template>
