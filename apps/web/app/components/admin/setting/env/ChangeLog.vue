<script setup lang="ts">
import { ENV_CHANGE } from '~/constants/settingEnv'

const api = useApi()
const page = ref(1)
const limit = 20

const { data, pending } = await useAsyncData(
  'admin-env-changes',
  async () => {
    const res = await api.get<{ items: EnvChange[]; total: number }>(
      `/admin/setting/env/changes?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page] }
)

const totalPages = computed(() =>
  Math.max(1, Math.ceil((data.value?.total ?? 0) / limit))
)
</script>

<template>
  <div class="space-y-3">
    <KunLoading v-if="pending" description="加载中..." />
    <template v-else-if="data.items.length">
      <div
        v-for="entry in data.items"
        :key="entry.id"
        class="border-default/20 bg-content1 space-y-1 rounded-lg border p-3"
      >
        <div class="flex flex-wrap items-center gap-2">
          <KunChip
            size="sm"
            variant="flat"
            :color="entry.change_kind === 'delete' ? 'danger' : 'primary'"
          >
            {{ ENV_CHANGE[entry.change_kind] }}
          </KunChip>
          <span class="font-mono text-sm font-medium break-all">
            {{ entry.variable }}
          </span>
          <template v-if="entry.changed_by">
            <KunAvatar :user="toKunUser(entry.changed_by)" size="sm" />
            <span class="text-sm">
              {{ entry.changed_by.name || `#${entry.changed_by.id}` }}
            </span>
          </template>
          <span v-else class="text-default-400 text-xs">导入工具</span>
          <span class="text-default-400 ml-auto text-xs">
            {{
              formatDate(entry.created_at, {
                isShowYear: true,
                isPrecise: true
              })
            }}
          </span>
        </div>
        <p class="font-mono text-xs break-all">
          <span class="text-default-500">{{
            entry.shown_before || '(无)'
          }}</span>
          →
          <span>{{ entry.shown_after || '(无)' }}</span>
        </p>
        <p v-if="entry.change_note" class="text-default-500 text-xs">
          {{ entry.change_note }}
        </p>
      </div>
      <div v-if="totalPages > 1" class="flex justify-center">
        <KunPagination v-model:current-page="page" :total-page="totalPages" />
      </div>
    </template>
    <KunNull v-else description="暂无配置中心变更记录" />
  </div>
</template>
