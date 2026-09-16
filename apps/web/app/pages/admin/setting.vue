<script setup lang="ts">
useKunDisableSeo('网站设置')

const api = useApi()

// No comment-verify. Comment pre-moderation was a moyu-only switch and the
// community primitive has none: a newcomer's first posts are held by ITS
// sandbox and released from ITS review queue.
type SettingKey = 'creator-only'

interface SettingDefinition {
  key: SettingKey
  name: string
  description: string
  isInverse: boolean
}

const definitions: SettingDefinition[] = [
  {
    key: 'creator-only',
    name: '仅创作者 / 版主 / 管理员可发布 Galgame',
    description:
      '开启后仅创作者 / 版主 / 管理员（及以上）可以发布补丁或提交新的 Galgame 条目，普通用户将被拒绝',
    isInverse: false
  }
]

const values = reactive<Record<SettingKey, boolean>>({
  'creator-only': false
})
const updating = reactive<Record<SettingKey, boolean>>({
  'creator-only': false
})

const { pending, refresh } = await useAsyncData('admin-setting', async () => {
  const results = await Promise.all(
    definitions.map(async (def) => {
      const res = await api.get<{ enabled?: boolean; disabled?: boolean }>(
        `/admin/setting/${def.key}`
      )
      if (res.code !== 0) return [def.key, false] as const
      const raw = def.isInverse ? res.data?.disabled : res.data?.enabled
      return [def.key, !!raw] as const
    })
  )
  for (const [key, value] of results) {
    values[key] = value
  }
  return values
})

const toggle = async (def: SettingDefinition) => {
  updating[def.key] = true
  try {
    const next = !values[def.key]
    const res = await api.put(`/admin/setting/${def.key}`, { enabled: next })
    if (res.code === 0) {
      values[def.key] = next
      useKunMessage('已更新', 'success')
    } else {
      useKunMessage(res.message || '更新失败', 'error')
    }
  } finally {
    updating[def.key] = false
  }
}

void refresh
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold">网站设置</h1>

    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-4">
      <KunCard v-for="def in definitions" :key="def.key" :bordered="true">
        <div class="flex items-center justify-between gap-4">
          <div class="flex-1">
            <h3 class="text-lg font-medium">{{ def.name }}</h3>
            <p class="text-default-500 text-sm">{{ def.description }}</p>
          </div>
          <KunSwitch
            :model-value="values[def.key]"
            :disabled="updating[def.key]"
            @update:model-value="toggle(def)"
          />
        </div>
      </KunCard>
    </div>
  </div>
</template>
