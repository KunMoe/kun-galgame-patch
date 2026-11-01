<script setup lang="ts">
import { USER_ROLE_MAP } from '~/constants/user'

const api = useApi()

const matrix = ref<RolePermissionMatrix | null>(null)
const working = ref<Record<string, string[]>>({})
const reason = ref('')
const loading = ref(true)
const saving = ref(false)

const apply = (m: RolePermissionMatrix) => {
  matrix.value = m
  working.value = Object.fromEntries(
    m.roles.map((layer) => [layer.role, [...layer.effective]])
  )
}

const load = async () => {
  loading.value = true
  const res = await api.get<RolePermissionMatrix>('/admin/permission')
  if (res.code === 0) {
    apply(res.data)
  } else {
    useKunMessage(res.message || '加载权限矩阵失败', 'error')
  }
  loading.value = false
}
onMounted(load)

const sections = computed(() =>
  (matrix.value?.groups ?? []).map((g) => ({
    ...g,
    entries: (matrix.value?.catalog ?? []).filter((e) => e.group === g.key)
  }))
)

const has = (role: string, key: string) =>
  working.value[role]?.includes(key) ?? false

const toggle = (role: string, key: string, value: boolean) => {
  const rest = (working.value[role] ?? []).filter((k) => k !== key)
  working.value = { ...working.value, [role]: value ? [...rest, key] : rest }
}

const cellState = (layer: RolePermissionLayer, key: string) => {
  const now = has(layer.role, key)
  if (now === layer.baseline.includes(key)) return null
  return now ? 'grant' : 'revoke'
}

const isPending = (layer: RolePermissionLayer, key: string) =>
  has(layer.role, key) !== layer.effective.includes(key)

const overridesFor = (layer: RolePermissionLayer): PermissionOverride[] =>
  (matrix.value?.catalog ?? []).flatMap((entry) => {
    const state = cellState(layer, entry.key)
    return state ? [{ permission: entry.key, effect: state }] : []
  })

const pendingCount = (layer: RolePermissionLayer) =>
  (matrix.value?.catalog ?? []).filter((e) => isPending(layer, e.key)).length

const dirtyRoles = computed(() =>
  (matrix.value?.roles ?? []).filter(
    (layer) => layer.can_edit && pendingCount(layer) > 0
  )
)

const pendingSensitive = computed(() =>
  (matrix.value?.catalog ?? []).filter(
    (e) => e.sensitive && dirtyRoles.value.some((l) => isPending(l, e.key))
  )
)

const moderatorExceedsAdmin = computed(() =>
  (matrix.value?.catalog ?? [])
    .filter((e) => has('moderator', e.key) && !has('admin', e.key))
    .map((e) => e.label)
)

const roleLabel = (role: string) => USER_ROLE_MAP[role] ?? role

const put = async (
  changes: { role: string; overrides: PermissionOverride[] }[]
) => {
  if (!matrix.value) return false
  saving.value = true
  const res = await api.put<RolePermissionMatrix>('/admin/permission/role', {
    version: matrix.value.version,
    reason: reason.value.trim(),
    changes
  })
  saving.value = false
  if (res.code === 0) {
    apply(res.data)
    reason.value = ''
    return true
  }
  useKunMessage(res.message || '保存失败', 'error')
  if (res.code === 40900) await load()
  return false
}

const save = async () => {
  const sensitive = pendingSensitive.value.map((e) => e.label)
  const ok = await useKunAlert({
    title: '保存权限调整',
    type: sensitive.length ? 'danger' : 'warning',
    message:
      `将调整：${dirtyRoles.value.map((l) => `${roleLabel(l.role)} ${pendingCount(l)} 项`).join('、')}。` +
      (sensitive.length
        ? `其中包含高风险权限：${sensitive.join('、')}。`
        : '') +
      '保存后持有这些角色的所有用户立即生效，确定继续？'
  })
  if (!ok) return
  const saved = await put(
    dirtyRoles.value.map((layer) => ({
      role: layer.role,
      overrides: overridesFor(layer)
    }))
  )
  if (saved) useKunMessage('已保存', 'success')
}

const reset = async (layer: RolePermissionLayer) => {
  const ok = await useKunAlert({
    title: `重置「${roleLabel(layer.role)}」为默认`,
    message: '将清除该角色的全部覆盖，回到代码基线，确定继续？'
  })
  if (!ok) return
  if (await put([{ role: layer.role, overrides: [] }])) {
    useKunMessage('已重置为默认', 'success')
  }
}

const discard = () => {
  if (matrix.value) apply(matrix.value)
}
</script>

<template>
  <div class="space-y-4">
    <KunInfo
      color="info"
      title="调整说明"
      description="勾选即授予该角色一项权限，取消即撤销。圆点标记相对代码基线的覆盖：绿色为额外授予，黄色为撤销。只能调整低于自己的角色，且只能动自己持有的权限；莲恒持全部权限。"
    />

    <KunInfo
      v-if="moderatorExceedsAdmin.length"
      color="warning"
      title="权限包含关系冲突"
      :description="`版主不能持有管理员没有的权限（后端会拒绝）：${moderatorExceedsAdmin.join('、')}`"
    />

    <KunLoading v-if="loading" description="正在加载权限矩阵..." />

    <template v-else-if="matrix">
      <KunCard :is-hoverable="false" padding="sm">
        <div class="overflow-x-auto">
          <table class="w-full min-w-xl text-sm">
            <thead>
              <tr class="border-default/20 border-b">
                <th class="py-2 pr-4 text-left font-medium">权限</th>
                <th
                  v-for="layer in matrix.roles"
                  :key="layer.role"
                  class="w-24 px-2 py-2 text-center font-medium"
                >
                  <div class="flex flex-col items-center gap-1">
                    <span>{{ roleLabel(layer.role) }}</span>
                    <KunButton
                      v-if="layer.can_edit && layer.overrides.length"
                      variant="light"
                      color="default"
                      size="xs"
                      :disabled="saving"
                      @click="reset(layer)"
                    >
                      重置
                    </KunButton>
                  </div>
                </th>
              </tr>
            </thead>
            <tbody v-for="section in sections" :key="section.key">
              <tr>
                <td
                  :colspan="matrix.roles.length + 1"
                  class="text-default-500 pt-4 pb-1 text-xs font-semibold"
                >
                  {{ section.label }}
                </td>
              </tr>
              <tr
                v-for="entry in section.entries"
                :key="entry.key"
                class="border-default/10 border-b last:border-b-0"
              >
                <td class="py-3 pr-4">
                  <div class="flex items-center gap-2 font-medium">
                    {{ entry.label }}
                    <KunChip
                      v-if="entry.sensitive"
                      color="danger"
                      size="xs"
                      variant="flat"
                    >
                      高风险
                    </KunChip>
                  </div>
                  <div class="text-default-500 text-xs">
                    {{ entry.description }}
                  </div>
                  <code class="text-default-400 text-xs">{{ entry.key }}</code>
                </td>
                <td
                  v-for="layer in matrix.roles"
                  :key="layer.role"
                  class="px-2 py-3"
                >
                  <div class="relative flex justify-center">
                    <KunCheckBox
                      color="primary"
                      :model-value="has(layer.role, entry.key)"
                      :disabled="!layer.can_edit || saving"
                      @update:model-value="
                        (v) => toggle(layer.role, entry.key, !!v)
                      "
                    />
                    <span
                      v-if="cellState(layer, entry.key)"
                      :class="
                        cn(
                          'absolute -top-1 right-5 size-1.5 rounded-full',
                          cellState(layer, entry.key) === 'grant'
                            ? 'bg-success'
                            : 'bg-warning'
                        )
                      "
                    />
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </KunCard>

      <KunCard :is-hoverable="false" padding="sm" class-name="space-y-3">
        <KunInput
          v-model="reason"
          placeholder="调整原因（可选，记入变更日志）"
          :maxlength="200"
        />
        <div class="flex flex-wrap items-center justify-between gap-3">
          <span class="text-default-500 text-sm">
            {{
              dirtyRoles.length
                ? `待保存：${dirtyRoles.map((l) => `${roleLabel(l.role)} ${pendingCount(l)} 项`).join('、')}`
                : '尚无待保存的调整'
            }}
          </span>
          <div class="flex gap-2">
            <KunButton
              variant="light"
              color="default"
              :disabled="!dirtyRoles.length || saving"
              @click="discard"
            >
              放弃更改
            </KunButton>
            <KunButton
              color="primary"
              :disabled="!dirtyRoles.length"
              :loading="saving"
              @click="save"
            >
              <KunIcon name="lucide:check" class="size-4" />
              保存
            </KunButton>
          </div>
        </div>
      </KunCard>
    </template>

    <KunNull v-else description="无法加载权限矩阵，请稍后重试" />
  </div>
</template>
