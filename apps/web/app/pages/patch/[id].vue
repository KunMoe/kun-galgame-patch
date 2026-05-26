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

// Writable computed so it can drive KunTab's v-model. The "set" path is a
// no-op because KunTab.href already calls navigateTo(); the route change
// re-runs the getter and the tab indicator follows.
const currentTab = computed({
  get: () => route.path.split('/').filter(Boolean).pop() ?? 'introduction',
  set: () => {}
})

// "编辑历史" / "编辑请求" tabs proxy the Wiki revision/PR surface that
// handbook §15 makes mandatory for moyu (pages/patch/[id]/revisions.vue,
// prs.vue).
const tabs = computed(() => [
  { key: 'introduction', title: 'Galgame 信息', href: `/patch/${galgameId.value}/introduction` },
  { key: 'resource', title: '补丁资源下载', href: `/patch/${galgameId.value}/resource` },
  { key: 'comment', title: '游戏评论', href: `/patch/${galgameId.value}/comment` },
  { key: 'revisions', title: '编辑历史', href: `/patch/${galgameId.value}/revisions` },
  { key: 'prs', title: '编辑请求', href: `/patch/${galgameId.value}/prs` }
])
</script>

<template>
  <div v-if="patch" class="mx-auto w-full max-w-7xl space-y-6 px-3 py-4">
    <!-- ── Hero ───────────────────────────────────────── -->
    <div
      class="border-default/20 relative overflow-hidden rounded-3xl border"
    >
      <div class="absolute inset-0">
        <!-- Use the 'mini' variant (460×259) for this decorative blurred
             backdrop. Saves 100-350 KB per page load vs the 1920×1080 main
             image — `blur-2xl scale-110` makes the resolution drop invisible.
             Matches the pattern resource/[id].vue already uses. -->
        <KunImage
          v-if="resolveBannerUrl(patch, 'mini')"
          :src="resolveBannerUrl(patch, 'mini')"
          :alt="displayName"
          class-name="block size-full"
          image-class-name="scale-110 blur-2xl"
        />
        <div
          class="from-background via-background/85 to-background/55 absolute inset-0 bg-gradient-to-t"
        />
      </div>

      <div class="relative flex flex-col gap-5 p-6 sm:flex-row sm:p-8">
        <div
          class="border-default/20 bg-default-100 aspect-video w-full shrink-0 overflow-hidden rounded-2xl border shadow-lg sm:w-72 lg:w-80"
        >
          <KunImage
            :src="resolveBannerUrl(patch) || '/kungalgame-trans.webp'"
            :alt="displayName"
            class-name="block size-full"
            image-class-name="transition-transform duration-300 hover:scale-[1.03]"
          />
        </div>

        <div class="flex min-w-0 flex-1 flex-col justify-between gap-4">
          <div class="space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <h1
                class="text-2xl leading-tight font-bold break-words sm:text-3xl"
              >
                {{ displayName }}
              </h1>
              <KunTooltip
                :text="GALGAME_AGE_LIMIT_DETAIL[patch.content_limit]"
                position="right"
              >
                <KunChip
                  :color="patch.content_limit === 'sfw' ? 'success' : 'danger'"
                  variant="flat"
                >
                  {{ GALGAME_AGE_LIMIT_MAP[patch.content_limit] }}
                </KunChip>
              </KunTooltip>
            </div>

            <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <template v-for="(value, key) in patch.name" :key="key">
                <span
                  v-if="value && value !== displayName"
                  class="text-default-500 text-xs"
                >
                  {{ value }}
                </span>
              </template>
            </div>

            <KunPatchAttribute
              :types="patch.type"
              :languages="patch.language"
              :platforms="patch.platform"
              size="sm"
            />
          </div>

          <div class="space-y-4">
            <div
              class="border-default/20 flex flex-col items-start justify-between gap-4 border-t pt-4 sm:flex-row sm:items-center"
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

            <PatchHeaderActions :patch="patch" />
          </div>
        </div>
      </div>
    </div>

    <!-- ── Tabs ───────────────────────────────────────── -->
    <KunTab
      v-model="currentTab"
      :items="tabs.map((t) => ({ value: t.key, textValue: t.title, href: t.href }))"
      variant="light"
      color="primary"
      size="md"
    />

    <div>
      <NuxtPage />
    </div>
  </div>

  <KunNull v-else description="Galgame 不存在" />
</template>
