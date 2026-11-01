<script setup lang="ts">
import { USER_ROLE_MAP } from '~/constants/user'

const api = useApi()

const catalog = ref<PermissionEntry[]>([])
const overridden = ref<PermissionOverriddenUser[]>([])

const query = ref('')
const searching = ref(false)
const candidates = ref<KunUser[]>([])

const loading = ref(false)
const saving = ref(false)
const view = ref<UserPermissionView | null>(null)
const working = ref<string[]>([])
const reason = ref('')

const labelOf = (key: string) =>
  catalog.value.find((e) => e.key === key)?.label ?? key

const loadOverridden = async () => {
  const res = await api.get<PermissionOverriddenUser[]>(
    '/admin/permission/users'
  )
  if (res.code === 0) overridden.value = res.data
}

onMounted(async () => {
  const res = await api.get<RolePermissionMatrix>('/admin/permission')
  if (res.code === 0) catalog.value = res.data.catalog
  await loadOverridden()
})

const apply = (v: UserPermissionView) => {
  view.value = v
  working.value = [...v.effective]
}

const open = async (uid: number) => {
  loading.value = true
  candidates.value = []
  const res = await api.get<UserPermissionView>(`/admin/permission/user/${uid}`)
  loading.value = false
  if (res.code === 0) {
    apply(res.data)
    reason.value = ''
  } else {
    view.value = null
    useKunMessage(res.message || '读取用户权限失败', 'error')
  }
}

const search = async () => {
  const q = query.value.trim()
  if (!q) return
  if (/^\d+$/.test(q)) {
    await open(Number(q))
    return
  }
  searching.value = true
  const res = await api.get<KunUser[]>(
    `/user/search?query=${encodeURIComponent(q.slice(0, 20))}`
  )
  searching.value = false
  candidates.value = res.code === 0 ? res.data : []
  if (!candidates.value.length) useKunMessage('没有找到该用户', 'warn')
}

const has = (key: string) => working.value.includes(key)

const toggle = (key: string, value: boolean) => {
  const rest = working.value.filter((k) => k !== key)
  working.value = value ? [...rest, key] : rest
}

const cellState = (key: string) => {
  if (!view.value) return null
  const now = has(key)
  if (now === view.value.baseline.includes(key)) return null
  return now ? 'grant' : 'revoke'
}

const dirty = computed(
  () =>
    !!view.value &&
    catalog.value.some(
      (e) => has(e.key) !== view.value!.effective.includes(e.key)
    )
)

const save = async (overrides: PermissionOverride[]) => {
  if (!view.value) return
  saving.value = true
  const res = await api.put<UserPermissionView>(
    `/admin/permission/user/${view.value.user.id}`,
    { version: view.value.version, reason: reason.value.trim(), overrides }
  )
  saving.value = false
  if (res.code === 0) {
    apply(res.data)
    reason.value = ''
    useKunMessage('已保存', 'success')
    await loadOverridden()
    return
  }
  useKunMessage(res.message || '保存失败', 'error')
  if (res.code === 40900) await open(view.value.user.id)
}

const saveWorking = () =>
  save(
    catalog.value.flatMap((e) => {
      const state = cellState(e.key)
      return state ? [{ permission: e.key, effect: state }] : []
    })
  )

const describe = (ovs: PermissionOverride[]) =>
  ovs
    .map((o) => `${o.effect === 'grant' ? '+' : '−'}${labelOf(o.permission)}`)
    .join('，')
</script>

<template>
  <div class="space-y-4">
    <KunInfo
      color="info"
      title="个人覆盖"
      description="个人覆盖优先于角色：可以给普通用户单独授予一项权限，也可以从某位版主身上单独撤销。只能调整角色低于自己的用户；持有莲的用户恒持全部权限。"
    />

    <div class="flex gap-2">
      <KunInput
        v-model="query"
        placeholder="用户名或用户 ID"
        class-name="max-w-64"
        @keyup.enter="search"
      />
      <KunButton
        color="primary"
        :loading="searching || loading"
        @click="search"
      >
        <KunIcon name="lucide:search" class="size-4" />
        查询
      </KunButton>
    </div>

    <div v-if="candidates.length" class="flex flex-wrap gap-2">
      <KunButton
        v-for="u in candidates"
        :key="u.id"
        variant="flat"
        color="default"
        size="sm"
        @click="open(u.id)"
      >
        <KunAvatar :user="toKunUser(u)" size="xs" />
        {{ u.name }}
        <span class="text-default-400">#{{ u.id }}</span>
      </KunButton>
    </div>

    <KunCard v-if="view" :is-hoverable="false" class-name="space-y-4">
      <div class="flex flex-wrap items-center gap-3">
        <KunAvatar :user="toKunUser(view.user)" size="md" />
        <div class="min-w-0">
          <div class="font-semibold">
            {{ view.user.name || `用户 #${view.user.id}` }}
          </div>
          <div class="text-default-500 flex flex-wrap gap-1 text-xs">
            <span>#{{ view.user.id }}</span>
            <KunChip
              v-for="r in view.roles"
              :key="r"
              size="xs"
              variant="flat"
              color="primary"
            >
              {{ USER_ROLE_MAP[r] ?? r }}
            </KunChip>
            <span v-if="!view.roles.length">普通用户</span>
          </div>
        </div>
      </div>

      <KunInfo
        v-if="!view.can_edit"
        color="warning"
        :description="
          view.locked
            ? '该用户持有莲，恒持全部权限，不可调整。'
            : '该用户的角色不低于你，不能调整其权限。'
        "
      />

      <div class="divide-default/10 divide-y">
        <div
          v-for="entry in catalog"
          :key="entry.key"
          class="flex items-center justify-between gap-4 py-3"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2 font-medium">
              {{ entry.label }}
              <KunChip
                v-if="entry.sensitive"
                color="danger"
                size="xs"
                variant="flat"
              >
                高风险
              </KunChip>
              <KunChip
                v-if="cellState(entry.key)"
                size="xs"
                variant="flat"
                :color="
                  cellState(entry.key) === 'grant' ? 'success' : 'warning'
                "
              >
                {{ cellState(entry.key) === 'grant' ? '个人授予' : '个人撤销' }}
              </KunChip>
            </div>
            <div class="text-default-500 text-xs">
              {{
                view.baseline.includes(entry.key)
                  ? '角色默认持有'
                  : '角色默认没有'
              }}
              · <code>{{ entry.key }}</code>
            </div>
          </div>
          <KunCheckBox
            color="primary"
            :model-value="has(entry.key)"
            :disabled="!view.can_edit || saving"
            @update:model-value="(v) => toggle(entry.key, !!v)"
          />
        </div>
      </div>

      <template v-if="view.can_edit">
        <KunInput
          v-model="reason"
          placeholder="调整原因（可选，记入变更日志）"
          :maxlength="200"
        />
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            color="default"
            :disabled="!view.overrides.length || saving"
            @click="save([])"
          >
            清除个人覆盖
          </KunButton>
          <KunButton
            color="primary"
            :disabled="!dirty"
            :loading="saving"
            @click="saveWorking"
          >
            <KunIcon name="lucide:check" class="size-4" />
            保存
          </KunButton>
        </div>
      </template>
    </KunCard>

    <section class="space-y-2">
      <h3 class="text-sm font-semibold">
        持有个人覆盖的用户（{{ overridden.length }}）
      </h3>
      <KunNull
        v-if="!overridden.length"
        description="目前没有任何用户持有个人覆盖"
      />
      <KunCard
        v-for="o in overridden"
        :key="o.user.id"
        clickable
        padding="sm"
        @click="open(o.user.id)"
      >
        <div class="flex items-center gap-3 text-sm">
          <KunAvatar :user="toKunUser(o.user)" size="sm" />
          <span class="font-medium">{{ o.user.name || `#${o.user.id}` }}</span>
          <span class="text-default-500 min-w-0 flex-1 truncate text-xs">
            {{ describe(o.overrides) }}
          </span>
        </div>
      </KunCard>
    </section>
  </div>
</template>
