<script setup lang="ts">
import { USER_ROLE_MAP } from '~/constants/user'

const api = useApi()
const page = ref(1)
const limit = 20
const role = ref('')
const userId = ref('')

const roleOptions = [
  { value: '', label: '全部角色' },
  ...['creator', 'moderator', 'admin'].map((r) => ({
    value: r,
    label: USER_ROLE_MAP[r] ?? r
  }))
]

const query = computed(() => {
  const q = new URLSearchParams({
    page: String(page.value),
    limit: String(limit)
  })
  if (role.value) q.set('role', role.value)
  const uid = Number(userId.value.trim())
  if (Number.isInteger(uid) && uid > 0) q.set('user_id', String(uid))
  return q.toString()
})

const { data, pending } = await useAsyncData(
  'admin-permission-changes',
  async () => {
    const res = await api.get<{ items: PermissionChange[]; total: number }>(
      `/admin/permission/change?${query.value}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [query] }
)

watch([role, userId], () => {
  page.value = 1
})

const totalPages = computed(() =>
  Math.max(1, Math.ceil((data.value?.total ?? 0) / limit))
)

const describe = (ovs: PermissionOverride[]) =>
  ovs.length
    ? ovs
        .map((o) => `${o.effect === 'grant' ? '+' : '−'} ${o.permission}`)
        .join('，')
    : '基线（无覆盖）'

const targetName = (c: PermissionChange) =>
  c.target_role
    ? `角色「${USER_ROLE_MAP[c.target_role] ?? c.target_role}」`
    : `用户「${c.target_user?.name || `#${c.target_user?.id}`}」`
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-wrap gap-2">
      <KunSelect
        v-model="role"
        :options="roleOptions"
        class-name="w-36"
        aria-label="按角色筛选"
      />
      <KunInput
        v-model="userId"
        placeholder="按用户 ID 筛选"
        class-name="w-40"
      />
    </div>

    <KunLoading v-if="pending" description="加载中..." />
    <template v-else-if="data.items.length">
      <div
        v-for="c in data.items"
        :key="c.id"
        class="border-default/20 bg-content1 shadow-kun-sm space-y-2 rounded-lg border p-3 text-sm"
      >
        <div class="flex flex-wrap items-center gap-2">
          <KunAvatar :user="toKunUser(c.actor)" size="sm" />
          <span class="font-medium">
            {{ c.actor.name || `#${c.actor.id}` }}
          </span>
          <span class="text-default-500">调整了 {{ targetName(c) }}</span>
          <span class="text-default-400 ml-auto text-xs">
            {{ formatDate(c.created, { isShowYear: true, isPrecise: true }) }}
          </span>
        </div>
        <div class="text-default-600 grid gap-1 text-xs sm:grid-cols-2">
          <div>
            <span class="text-default-400">之前：</span>{{ describe(c.before) }}
          </div>
          <div>
            <span class="text-default-400">之后：</span>{{ describe(c.after) }}
          </div>
        </div>
        <p v-if="c.reason" class="text-default-500 text-xs">
          原因：{{ c.reason }}
        </p>
      </div>
      <div v-if="totalPages > 1" class="flex justify-center">
        <KunPagination v-model:current-page="page" :total-page="totalPages" />
      </div>
    </template>
    <KunNull v-else description="暂无权限变更记录" />
  </div>
</template>
