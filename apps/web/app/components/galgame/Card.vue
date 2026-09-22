<script setup lang="ts">
import { SUPPORTED_TYPE, SUPPORTED_TYPE_SHORT_MAP } from '~/constants/resource'

interface Props {
  patch: GalgameCard
}

const props = defineProps<Props>()

const settingStore = useSettingStore()
const titleLanguage = computed(() => settingStore.data.titleLanguage ?? 'ja-jp')
const showJapaneseSubtitle = computed(
  () => settingStore.data.showJapaneseSubtitle ?? false
)
const showReleaseDate = computed(
  () => settingStore.data.showReleaseDate ?? false
)
const showNsfwBadge = computed(() => settingStore.data.showNsfwBadge ?? false)

const galgameName = computed(() =>
  getPreferredLanguageText(props.patch.name, titleLanguage.value)
)

const japaneseSubtitle = computed(() => {
  if (!showJapaneseSubtitle.value) return ''
  const ja = props.patch.name?.['ja-jp'] ?? ''
  return ja && ja !== galgameName.value ? ja : ''
})

// Portrait only. catalog fills covers.portrait from whatever cover the work
// has, so this is not guaranteed to be taller than wide — the 5/7 box crops it.
const coverSrc = computed(() => resolvePortraitUrl(props.patch))

const releaseDate = computed(() => props.patch.release_date?.slice(0, 10) ?? '')

const patchHref = computed(() => `/galgame/${props.patch.id}`)

const maker = computed(() => resolveMaker(props.patch))
const makerName = computed(() =>
  getPreferredLanguageText(maker.value?.name, titleLanguage.value)
)
const makerHref = computed(() =>
  maker.value ? `/galgame/official/${maker.value.id}` : ''
)

// resource_update_time is what /galgame sorts by, so the card shows the same
// clock the order is drawn from. A library row has no patch and no such time.
const updatedAt = computed(
  () => props.patch.resource_update_time || props.patch.created
)

const isNoPatch = computed(() => (props.patch.count?.resource ?? 0) === 0)

const isNsfw = computed(() => props.patch.content_limit !== 'sfw')

const MAX_TYPE_BADGES = 3

// patch.type is the union of every resource's type in publish order, so the
// same game reads 修图 first on one card and 汉化 first on the next. Ranking by
// SUPPORTED_TYPE puts the translation patches — what a reader scans for — in
// front of the overflow count.
const typeBadges = computed(() => {
  const ordered = [...props.patch.type].sort(
    (a, b) => SUPPORTED_TYPE.indexOf(a) - SUPPORTED_TYPE.indexOf(b)
  )
  return {
    shown: ordered
      .slice(0, MAX_TYPE_BADGES)
      .map((t) => SUPPORTED_TYPE_SHORT_MAP[t] ?? t),
    rest: Math.max(0, ordered.length - MAX_TYPE_BADGES)
  }
})
</script>

<template>
  <div class="flex h-full flex-col">
    <KunNsfwMask :nsfw="isNsfw" rounded="rounded-lg">
      <NuxtLink
        :to="patchHref"
        :aria-label="galgameName"
        tabindex="-1"
        class="relative block overflow-hidden rounded-lg"
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
          class="bg-default-100 text-default-400 flex items-center justify-center"
          style="aspect-ratio: 5 / 7"
        >
          <KunIcon name="lucide:image-off" class="size-6" />
        </div>

        <div
          class="absolute top-1.5 left-1.5 flex flex-wrap gap-1"
          :class="showNsfwBadge ? 'right-7' : 'right-1.5'"
        >
          <KunChip v-if="isNoPatch" color="default" variant="solid" size="xs">
            暂无补丁
          </KunChip>
          <template v-else>
            <KunChip
              v-for="label in typeBadges.shown"
              :key="label"
              color="primary"
              variant="solid"
              size="xs"
            >
              {{ label }}
            </KunChip>
            <KunChip
              v-if="typeBadges.rest"
              color="default"
              variant="solid"
              size="xs"
            >
              +{{ typeBadges.rest }}
            </KunChip>
          </template>
        </div>

        <div
          v-if="showNsfwBadge"
          class="absolute top-0 right-0 size-5 [clip-path:polygon(100%_0,100%_100%,0_0)]"
          :class="
            props.patch.content_limit === 'sfw' ? 'bg-success' : 'bg-danger'
          "
          :title="props.patch.content_limit.toLocaleUpperCase()"
        />

        <!-- SANCTIONED EXCEPTION to 铁律 #1 (no gradients): a bottom-to-top black
           scrim so the counts stay legible over an arbitrary cover. Same one
           the forum card carries; listed in CLAUDE.md. Do NOT remove it in a
           no-gradient sweep. -->
        <div
          v-if="!isNoPatch"
          class="absolute right-0 bottom-0 left-0 flex items-center gap-2 bg-gradient-to-t from-black/70 to-transparent px-2 pt-4 pb-1.5 text-xs text-white"
        >
          <span class="flex shrink-0 items-center gap-1">
            <KunIcon name="lucide:eye" class="size-3.5 text-inherit" />
            {{ formatNumber(props.patch.view) }}
          </span>
          <span class="flex shrink-0 items-center gap-1">
            <KunIcon name="lucide:download" class="size-3.5 text-inherit" />
            {{ formatNumber(props.patch.download) }}
          </span>
          <span class="ml-auto flex shrink-0 items-center gap-1">
            <KunIcon name="lucide:puzzle" class="size-3.5 text-inherit" />
            {{ formatNumber(props.patch.count.resource) }}
          </span>
        </div>
      </NuxtLink>
    </KunNsfwMask>

    <div class="flex flex-auto flex-col pt-1.5">
      <h2 class="line-clamp-2 text-sm font-medium">
        <NuxtLink
          :to="patchHref"
          class="hover:text-primary-500 transition-colors"
        >
          {{ galgameName }}
        </NuxtLink>
      </h2>

      <p
        v-if="japaneseSubtitle"
        class="text-default-500 mt-0.5 line-clamp-1 text-xs"
      >
        {{ japaneseSubtitle }}
      </p>

      <p
        v-if="showReleaseDate && releaseDate"
        class="text-default-500 mt-0.5 text-xs"
      >
        {{ releaseDate }} 发售
      </p>

      <slot name="meta" />

      <div
        v-if="makerName || !isNoPatch"
        class="text-default-500 mt-auto flex min-w-0 items-center gap-1.5 pt-1.5 text-xs"
      >
        <NuxtLink
          v-if="makerName"
          :to="makerHref"
          class="hover:text-primary-500 truncate transition-colors"
          :title="makerName"
        >
          {{ makerName }}
        </NuxtLink>
        <span v-if="!isNoPatch && updatedAt" class="ml-auto shrink-0">
          {{ formatDistanceToNow(updatedAt) }}
        </span>
      </div>
    </div>
  </div>
</template>
