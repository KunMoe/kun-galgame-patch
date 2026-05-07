<script setup lang="ts">
import {
  GALGAME_AGE_LIMIT_DETAIL,
  GALGAME_AGE_LIMIT_MAP
} from '~/constants/galgame'

const route = useRoute()
const api = useApi()

const galgameId = computed(() => Number(route.params.id))

const { data: patch } = await useAsyncData<PatchHeader | null>(
  () => `patch-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchHeader>(`/patch/${galgameId.value}`)
    return res.code === 0 ? res.data : null
  }
)

const displayName = computed(() =>
  patch.value ? getPreferredLanguageText(patch.value.name) : ''
)

useKunSeoMeta({
  title: displayName.value || `补丁 ${galgameId.value}`,
  description: displayName.value ? `${displayName.value} 的补丁下载` : ''
})

onMounted(async () => {
  await api.put(`/patch/${galgameId.value}/view`).catch(() => {})
})

provide('patch', patch)

const currentTab = computed(() => {
  const last = route.path.split('/').filter(Boolean).pop() ?? 'introduction'
  return last
})

const tabs = computed(() => [
  { key: 'introduction', title: 'Galgame 信息', href: `/patch/${galgameId.value}/introduction` },
  { key: 'resource', title: '补丁资源下载', href: `/patch/${galgameId.value}/resource` },
  { key: 'comment', title: '游戏评论', href: `/patch/${galgameId.value}/comment` }
])
</script>

<template>
  <div v-if="patch" class="w-full space-y-6">
    <div class="relative mx-auto w-full max-w-7xl">
      <div
        v-if="patch.banner"
        class="bg-default-100 pointer-events-none absolute inset-x-0 top-0 -z-10 h-64 overflow-hidden"
      >
        <img
          :src="patch.banner"
          :alt="displayName"
          class="h-full w-full object-cover blur-2xl brightness-75 opacity-60"
        />
      </div>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div
          class="bg-default-100 aspect-video overflow-hidden rounded-2xl md:col-span-1"
        >
          <img
            :src="patch.banner || '/kungalgame-trans.webp'"
            :alt="displayName"
            class="h-full w-full object-cover"
          />
        </div>

        <div class="flex flex-col gap-3 md:col-span-2 md:px-6">
          <div class="flex flex-wrap items-center gap-2">
            <h1 class="text-2xl leading-tight font-bold sm:text-3xl">
              {{ displayName }}
            </h1>
            <KunTooltip
              :text="GALGAME_AGE_LIMIT_DETAIL[patch.content_limit]"
              position="right"
            >
              <KunBadge
                :color="patch.content_limit === 'sfw' ? 'success' : 'danger'"
                variant="flat"
              >
                {{ GALGAME_AGE_LIMIT_MAP[patch.content_limit] }}
              </KunBadge>
            </KunTooltip>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <span
              v-for="(value, key) in patch.name"
              :key="key"
              class="text-default-500 text-xs"
            >
              <template v-if="value && value !== displayName">{{ value }}</template>
            </span>
          </div>

          <KunPatchAttribute
            :types="patch.type"
            :languages="patch.language"
            :platforms="patch.platform"
            size="sm"
          />

          <KunDivider color="default" />

          <div
            class="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center"
          >
            <KunUser
              :user="patch.user"
              :description="`资源更新于 ${formatDistanceToNow(patch.resource_update_time)}`"
            />
            <KunCardStats
              :patch="{ ...patch, created: patch.created }"
              :disable-tooltip="false"
              :is-mobile="false"
            />
          </div>
        </div>
      </div>

      <nav class="border-default/20 mt-6 flex gap-4 border-b">
        <NuxtLink
          v-for="t in tabs"
          :key="t.key"
          :to="t.href"
          :class="
            cn(
              'px-3 py-3 text-sm transition-colors',
              currentTab === t.key
                ? 'text-primary border-primary -mb-px border-b-2 font-medium'
                : 'text-default-600 hover:text-foreground'
            )
          "
        >
          {{ t.title }}
        </NuxtLink>
      </nav>
    </div>

    <div class="mx-auto max-w-7xl px-3">
      <NuxtPage />
    </div>
  </div>

  <KunNull v-else description="Galgame 不存在" />
</template>
