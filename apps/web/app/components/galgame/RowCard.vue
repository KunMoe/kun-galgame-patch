<script setup lang="ts">
import {
  SUPPORTED_TYPE,
  SUPPORTED_TYPE_MAP,
  SUPPORTED_TYPE_SHORT_MAP
} from '~/constants/resource'

interface Props {
  patch: GalgameCard
}

const props = defineProps<Props>()

const settingStore = useSettingStore()
const titleLanguage = computed(() => settingStore.data.titleLanguage ?? 'ja-jp')
const showNsfwBadge = computed(() => settingStore.data.showNsfwBadge ?? false)

const galgameName = computed(() =>
  getPreferredLanguageText(props.patch.name, titleLanguage.value)
)

// The row is wide enough to carry both names, so the Japanese title is not
// behind the poster card's 显示日文标题 toggle — it appears whenever the reader
// is reading titles in some other language and the work has a different one.
const japaneseName = computed(() => {
  if (titleLanguage.value === 'ja-jp') return ''
  const ja = props.patch.name?.['ja-jp'] ?? ''
  return ja && ja !== galgameName.value ? ja : ''
})

const coverSrc = computed(() => resolvePortraitUrl(props.patch))

const patchHref = computed(() => `/galgame/${props.patch.id}`)

const maker = computed(() => resolveMaker(props.patch))
const makerName = computed(() =>
  getPreferredLanguageText(maker.value?.name, titleLanguage.value)
)
const makerHref = computed(() =>
  maker.value ? `/galgame/official/${maker.value.id}` : ''
)

const releaseDate = computed(() => props.patch.release_date?.slice(0, 10) ?? '')

const updatedAt = computed(
  () => props.patch.resource_update_time || props.patch.created
)

const isNoPatch = computed(() => (props.patch.count?.resource ?? 0) === 0)

const isNsfw = computed(() => props.patch.content_limit !== 'sfw')

const MAX_TYPE_BADGES = 3

// Same ranking the poster card uses: patch.type is the union of every
// resource's type in publish order, so without it the same game reads 修图
// first on one row and 汉化 first on the next. The short labels stay even
// though this column is wider than a poster corner — three full names wrap
// the row — so each badge carries the full one as its tooltip.
const typeBadges = computed(() => {
  const ordered = [...(props.patch.type ?? [])].sort(
    (a, b) => SUPPORTED_TYPE.indexOf(a) - SUPPORTED_TYPE.indexOf(b)
  )
  return {
    shown: ordered.slice(0, MAX_TYPE_BADGES).map((type) => ({
      type,
      label: SUPPORTED_TYPE_SHORT_MAP[type] ?? type,
      full: SUPPORTED_TYPE_MAP[type] ?? type
    })),
    rest: Math.max(0, ordered.length - MAX_TYPE_BADGES)
  }
})

const staffLines = computed(() => {
  const facet = props.patch.facet
  const line = (label: string, people?: KunLanguage[]) => {
    const names = (people ?? [])
      .map((n) => getPreferredLanguageText(n, titleLanguage.value))
      .filter(Boolean)
    return names.length ? { label, value: names.join(' / ') } : null
  }
  return [
    line('剧本', facet?.scenario),
    line('原画', facet?.illustration)
  ].filter((l) => l !== null)
})

const tags = computed(() => props.patch.facet?.tags ?? [])

// A one-column row is barely wider than its cover, and the full shelf wraps to
// three lines there while the cover beside it stops after one.
const NARROW_TAG_LIMIT = 3
</script>

<template>
  <div class="@container/row flex h-full gap-3">
    <KunNsfwMask
      :nsfw="isNsfw"
      rounded="rounded-md"
      class="w-24 shrink-0 self-start sm:w-28"
    >
      <NuxtLink
        :to="patchHref"
        :aria-label="galgameName"
        tabindex="-1"
        class="relative block overflow-hidden rounded-md"
        :class="!coverSrc && 'bg-default-100'"
      >
        <!-- The zoom rides the wrapper: on image-class-name tailwind-merge
           replaces KunUI's own transition-opacity on the <img> and the
           thumbhash blur-up stops fading in. -->
        <KunImage
          v-if="coverSrc"
          :src="coverSrc"
          :alt="galgameName"
          aspect-ratio="5 / 7"
          :thumbhash="resolvePortraitThumbhash(props.patch)"
          class-name="transition-transform duration-300 hover:scale-105"
        />
        <div
          v-else
          class="text-default-400 flex items-center justify-center"
          style="aspect-ratio: 5 / 7"
        >
          <KunIcon name="lucide:image-off" class="size-6" />
        </div>

        <div
          v-if="showNsfwBadge"
          class="absolute top-0 right-0 size-5 [clip-path:polygon(100%_0,100%_100%,0_0)]"
          :class="
            props.patch.content_limit === 'sfw' ? 'bg-success' : 'bg-danger'
          "
          :title="props.patch.content_limit.toLocaleUpperCase()"
        />
      </NuxtLink>
    </KunNsfwMask>

    <div class="flex min-w-0 flex-1 flex-col gap-1.5">
      <div class="min-w-0">
        <h2 class="line-clamp-2 leading-snug font-medium">
          <NuxtLink
            :to="patchHref"
            class="hover:text-primary-500 transition-colors"
          >
            {{ galgameName }}
          </NuxtLink>
        </h2>
        <p
          v-if="japaneseName"
          class="text-default-500 line-clamp-1 text-xs"
          :title="japaneseName"
        >
          {{ japaneseName }}
        </p>
      </div>

      <div class="flex flex-wrap gap-1">
        <KunChip v-if="isNoPatch" color="default" variant="flat" size="xs">
          暂无补丁
        </KunChip>
        <template v-else>
          <KunChip
            v-for="badge in typeBadges.shown"
            :key="badge.type"
            color="primary"
            variant="flat"
            size="xs"
            :title="badge.full"
          >
            {{ badge.label }}
          </KunChip>
          <KunChip
            v-if="typeBadges.rest"
            color="default"
            variant="flat"
            size="xs"
          >
            +{{ typeBadges.rest }}
          </KunChip>
        </template>
      </div>

      <div
        v-if="makerName || releaseDate"
        class="text-default-500 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs"
      >
        <NuxtLink
          v-if="makerName"
          :to="makerHref"
          class="hover:text-primary-500 flex min-w-0 items-center gap-1 transition-colors"
          :title="makerName"
        >
          <KunIcon name="lucide:building-2" class="size-3.5 shrink-0" />
          <span class="truncate">{{ makerName }}</span>
        </NuxtLink>
        <span v-if="releaseDate" class="flex shrink-0 items-center gap-1">
          <KunIcon name="lucide:calendar" class="size-3.5" />
          {{ releaseDate }}
        </span>
      </div>

      <slot name="meta" />

      <div v-if="staffLines.length" class="space-y-0.5 text-xs">
        <p v-for="line in staffLines" :key="line.label" class="flex gap-1.5">
          <span class="text-default-400 shrink-0">{{ line.label }}</span>
          <span class="text-default-600 truncate" :title="line.value">
            {{ line.value }}
          </span>
        </p>
      </div>

      <div v-if="tags.length" class="flex flex-wrap gap-1">
        <NuxtLink
          v-for="(tag, index) in tags"
          :key="tag.id"
          :to="`/galgame/tag/${tag.id}`"
          :class="
            cn(
              'bg-default-100 text-default-600 hover:bg-primary/15 hover:text-primary rounded px-1.5 py-0.5 text-xs transition-colors',
              index >= NARROW_TAG_LIMIT && 'hidden @md/row:block'
            )
          "
        >
          {{ tag.name }}
        </NuxtLink>
      </div>

      <div
        class="text-default-500 mt-auto flex items-center gap-3 pt-1 text-xs"
      >
        <template v-if="!isNoPatch">
          <span class="flex shrink-0 items-center gap-1">
            <KunIcon name="lucide:eye" class="size-3.5" />
            {{ formatNumber(props.patch.view) }}
          </span>
          <span class="flex shrink-0 items-center gap-1">
            <KunIcon name="lucide:download" class="size-3.5" />
            {{ formatNumber(props.patch.download) }}
          </span>
          <span class="flex shrink-0 items-center gap-1">
            <KunIcon name="lucide:puzzle" class="size-3.5" />
            {{ formatNumber(props.patch.count.resource) }}
          </span>
          <span v-if="updatedAt" class="ml-auto shrink-0">
            {{ formatDistanceToNow(updatedAt) }}
          </span>
        </template>
        <span v-else-if="props.patch.count?.favorite_by" class="shrink-0">
          {{ formatNumber(props.patch.count.favorite_by) }} 人想要
        </span>
      </div>
    </div>
  </div>
</template>
