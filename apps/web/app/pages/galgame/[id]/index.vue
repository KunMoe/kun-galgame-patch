<script setup lang="ts">
import {
  GALGAME_AGE_LIMIT_DETAIL,
  GALGAME_AGE_LIMIT_MAP
} from '~/constants/galgame'
import {
  SUPPORTED_TYPE_MAP,
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM_MAP
} from '~/constants/resource'
import { kunMoyuMoe } from '~/config/moyu-moe'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const settingStore = useSettingStore()

const galgameId = computed(() => Number(route.params.id))

const coversOpen = ref(false)

const { data: patch } = await useAsyncData<PatchHeader | null>(
  () => `patch-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchHeader>(`/patch/${galgameId.value}`)
    return res.code === 0 ? res.data : null
  }
)

const chipUserId = computed(
  () => patch.value?.creator?.id ?? patch.value?.user?.id ?? 0
)
const { data: chipUserInfo } = await useAsyncData<UserInfo | null>(
  () => `patch-chip-user-${chipUserId.value}`,
  async () => {
    if (!chipUserId.value) return null
    const res = await api.get<UserInfo>(`/user/${chipUserId.value}`)
    return res.code === 0 ? res.data : null
  },
  { watch: [chipUserId] }
)
const creatorDescription = computed(
  () => `已发布 ${chipUserInfo.value?.resource_count ?? 0} 个游戏补丁`
)

const shouldShowNsfwConfirm = computed(() => {
  if (patch.value) return false
  if (userStore.user.id > 0) return false
  if (settingStore.data.kunNsfwEnable !== 'sfw') return false
  if (settingStore.isNsfwAcked(galgameId.value)) return false
  return true
})

const confirmNsfw = () => {
  settingStore.ackNsfw(galgameId.value)
  if (import.meta.client) location.reload()
}

const displayName = computed(() =>
  patch.value ? getPreferredLanguageText(patch.value.name) : ''
)

// The header box is 3/4 and crops, so it wants catalog's portrait slot. That
// slot is filled from ANY cover when the work has no portrait-shaped one, so
// falling back to the banner only matters for works with no cover at all.
const heroSrc = computed(
  () =>
    resolvePortraitUrl(patch.value) ||
    resolveBannerUrl(patch.value) ||
    '/kungalgame-trans.webp'
)

const heroThumbhash = computed(() =>
  resolvePortraitUrl(patch.value)
    ? resolvePortraitThumbhash(patch.value)
    : resolveBannerThumbhash(patch.value)
)

const releaseDate = computed(
  () => patch.value?.release_date?.slice(0, 10) ?? ''
)

const isOnSite = computed(() => patch.value?.is_on_forum !== false)
const hasResources = computed(() => (patch.value?.count?.resource ?? 0) > 0)

const isR18Patch = patch.value?.type?.includes('r18') ?? false
if (
  patch.value &&
  patch.value.content_limit === 'sfw' &&
  !isR18Patch &&
  patch.value.indexed
) {
  const p = patch.value
  const base = displayName.value || `补丁 ${galgameId.value}`
  const cover = resolveBannerUrl(p) || undefined

  const canonicalPath = `/patch/${galgameId.value}/resource`
  const canonicalUrl = `${kunMoyuMoe.domain.main}${canonicalPath}`

  const jaName = p.name['ja-jp']
  const title = jaName && jaName !== base ? `${base} | ${jaName}` : base

  const typeLabels = p.type.map((t) => SUPPORTED_TYPE_MAP[t]).filter(Boolean)
  const langLabels = p.language
    .map((l) => SUPPORTED_LANGUAGE_MAP[l])
    .filter(Boolean)
  const platLabels = p.platform
    .map((pl) => SUPPORTED_PLATFORM_MAP[pl])
    .filter(Boolean)
  const description =
    `《${base}》补丁资源下载。` +
    `${typeLabels.length ? `提供${typeLabels.slice(0, 5).join('、')}` : '提供汉化补丁等资源'}` +
    `${platLabels.length ? `，支持 ${platLabels.join('、')}` : ''}` +
    `${langLabels.length ? `，语言 ${langLabels.join('、')}` : ''}。` +
    `开源免费、CDN 加速下载。`

  useKunSeoMeta(
    {
      title,
      description,
      ogType: 'article',
      ...(cover && { ogImage: cover }),
      articlePublishedTime: new Date(p.created).toISOString(),
      articleModifiedTime: new Date(p.resource_update_time).toISOString()
    },
    undefined,
    canonicalPath
  )

  const alternateNames = (['ja-jp', 'en-us', 'zh-cn', 'zh-tw'] as const)
    .map((l) => p.name[l])
    .filter((n): n is string => !!n && n !== base)
  const videoGameLd: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'VideoGame',
    name: base,
    url: canonicalUrl,
    datePublished: p.release_date || new Date(p.created).toISOString(),
    dateModified: new Date(p.resource_update_time).toISOString(),
    ...(alternateNames.length && { alternateName: alternateNames }),
    ...(cover && { image: cover }),
    ...(p.galgame?.original_language && {
      inLanguage: p.galgame.original_language
    }),
    ...(platLabels.length && { gamePlatform: platLabels }),
    ...(typeLabels.length && { keywords: typeLabels.join(', ') }),
    interactionStatistic: [
      {
        '@type': 'InteractionCounter',
        interactionType: { '@type': 'WatchAction' },
        userInteractionCount: p.view
      },
      {
        '@type': 'InteractionCounter',
        interactionType: { '@type': 'LikeAction' },
        userInteractionCount: p.count.favorite_by
      }
    ]
  }
  const breadcrumbLd: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      {
        '@type': 'ListItem',
        position: 1,
        name: '首页',
        item: kunMoyuMoe.domain.main
      },
      {
        '@type': 'ListItem',
        position: 2,
        name: 'Galgame 补丁',
        item: `${kunMoyuMoe.domain.main}/galgame`
      },
      { '@type': 'ListItem', position: 3, name: base, item: canonicalUrl }
    ]
  }
  useHead({
    script: [
      {
        id: 'schema-org-video-game',
        type: 'application/ld+json',
        innerHTML: JSON.stringify(videoGameLd)
      },
      {
        id: 'schema-org-breadcrumb',
        type: 'application/ld+json',
        innerHTML: JSON.stringify(breadcrumbLd)
      }
    ]
  })
} else {
  useKunDisableSeo(displayName.value || `补丁 ${galgameId.value}`)
}

onMounted(async () => {
  if (!isOnSite.value) return
  await api.put(`/patch/${galgameId.value}/view`).catch(() => {})
})

provide('patch', patch)

const tabs = computed(() => {
  const all = [
    {
      key: 'introduction',
      title: 'Galgame 信息',
      href: `/patch/${galgameId.value}/introduction`
    },
    {
      key: 'resource',
      title: '补丁资源下载',
      href: `/patch/${galgameId.value}/resource`
    },
    {
      key: 'comment',
      title: '游戏评论',
      href: `/patch/${galgameId.value}/comment`
    }
  ]
  return all
})

const currentTab = computed({
  get: () => {
    const segment = route.path.split('/').filter(Boolean).pop() ?? ''
    return tabs.value.some((t) => t.key === segment) ? segment : 'introduction'
  },
  set: () => {}
})
</script>

<template>
  <div v-if="patch" class="mx-auto w-full max-w-7xl space-y-6 px-3 py-4">
    <div class="flex flex-col gap-6 sm:flex-row">
      <div class="mx-auto w-40 shrink-0 space-y-2 sm:mx-0 sm:w-44 lg:w-52">
        <KunLightboxGallery>
          <KunLightboxGalleryItem
            :src="heroSrc"
            :alt="displayName"
            as="div"
            class="border-default/20 bg-default-100 w-full overflow-hidden rounded-2xl border shadow-lg"
          >
            <KunImage
              :src="heroSrc"
              :alt="displayName"
              loading="eager"
              fetchpriority="high"
              aspect-ratio="3/4"
              object-fit="cover"
              :thumbhash="heroThumbhash"
              class-name="block w-full"
              image-class-name="transition-transform duration-300 hover:scale-[1.03]"
            />
          </KunLightboxGalleryItem>
        </KunLightboxGallery>

        <KunButton
          variant="light"
          color="default"
          size="sm"
          class-name="w-full"
          @click="coversOpen = true"
        >
          <KunIcon name="lucide:images" class="size-4" />
          查看所有封面
        </KunButton>
        <GalgameCovers v-model="coversOpen" :galgame-id="galgameId" />
      </div>

      <div class="flex min-w-0 flex-1 flex-col gap-4">
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

          <div
            v-if="releaseDate"
            class="text-default-500 flex items-center gap-1.5 text-sm"
          >
            <KunIcon name="lucide:calendar" class="size-4" />
            <span>{{ releaseDate }} 发售</span>
          </div>
        </div>

        <div class="space-y-4">
          <div
            class="border-default/20 flex flex-col items-start justify-between gap-4 border-t pt-4 sm:flex-row sm:items-center"
          >
            <KunUserChip
              :user="patch.creator ?? patch.user"
              :description="creatorDescription"
            />
            <KunCardStats
              v-if="hasResources"
              :patch="{ ...patch, created: patch.created }"
              :disable-tooltip="false"
              :is-mobile="false"
            />
            <KunChip v-else color="default" variant="flat" size="sm">
              暂无补丁
            </KunChip>
          </div>

          <PatchHeaderActions :patch="patch" />
        </div>
      </div>
    </div>

    <KunTab
      v-model="currentTab"
      :items="
        tabs.map((t) => ({ value: t.key, textValue: t.title, href: t.href }))
      "
      variant="underlined"
      color="primary"
      size="md"
    />

    <div>
      <NuxtPage />
    </div>
  </div>

  <div
    v-else-if="shouldShowNsfwConfirm"
    class="mx-auto flex w-full max-w-xl flex-col items-center gap-6 px-4 py-16 text-center"
  >
    <div
      class="bg-danger/10 text-danger flex size-16 items-center justify-center rounded-full"
    >
      <KunIcon name="lucide:shield-alert" class="size-8" />
    </div>
    <div class="space-y-2">
      <h1 class="text-2xl font-bold">该 Galgame 含有 NSFW 内容</h1>
      <p class="text-default-500 text-sm leading-relaxed">
        您需要点击下方按钮以确认查看。<br />
        登录后无需每次确认。
      </p>
    </div>
    <div class="flex flex-col gap-2 sm:flex-row">
      <KunButton color="danger" size="md" @click="confirmNsfw">
        我已知晓，仍要查看
      </KunButton>
      <NuxtLink to="/">
        <KunButton variant="light" color="default" size="md">
          返回首页
        </KunButton>
      </NuxtLink>
    </div>
  </div>

  <KunNull v-else description="Galgame 不存在" />
</template>
