<script setup lang="ts">
import { GALGAME_AGE_LIMIT_MAP } from '~/constants/galgame'

interface Props {
  patch: GalgameCard
}

const props = defineProps<Props>()

// Card display preferences (the /galgame "显示设置" panel; cookie-persisted).
// `?? default` guards an older cookie written before these keys existed.
const settingStore = useSettingStore()
const titleLanguage = computed(() => settingStore.data.titleLanguage ?? 'ja-jp')
const showJapaneseSubtitle = computed(
  () => settingStore.data.showJapaneseSubtitle ?? false
)
const showReleaseDate = computed(() => settingStore.data.showReleaseDate ?? false)
const showNsfwBadge = computed(() => settingStore.data.showNsfwBadge ?? true)

const galgameName = computed(() =>
  getPreferredLanguageText(props.patch.name, titleLanguage.value)
)

// Opt-in Japanese subtitle under the title — suppressed when it would just
// duplicate the title (title already 日语, or a zh title that fell back to ja).
const japaneseSubtitle = computed(() => {
  if (!showJapaneseSubtitle.value) return ''
  const ja = props.patch.name?.['ja-jp'] ?? ''
  return ja && ja !== galgameName.value ? ja : ''
})

const bannerSrc = computed(
  () => resolveBannerUrl(props.patch, 'mini') || '/kungalgame-trans.webp'
)

// Release date as YYYY-MM-DD. Backend sends RFC3339 ("2016-11-25T00:00:00Z");
// slicing the first 10 chars is enough for a day-precision date and avoids a
// date-lib dependency + timezone shift. Null/absent → not shown.
const releaseDate = computed(() => props.patch.release_date?.slice(0, 10) ?? '')
</script>

<template>
  <!-- `p-0` removes KunCard's default outer padding so the banner image is
       flush with the card edges; the inner info section keeps its own p-3
       so text doesn't touch the rounded corners. `content-class="gap-0 p-0"`
       likewise zeroes the wrapper KunCard puts around <slot />. -->
  <KunCard
    :href="`/patch/${props.patch.id}/introduction`"
    class-name="w-full p-0"
    content-class="gap-0 p-0"
  >
    <div class="relative mx-auto w-full text-center">
      <!-- KunImage owns the skeleton + fade-in via useImageLoadingStatus.
           Don't add a sibling skeleton or `@load` listener on the component
           root — KunImage doesn't emit `load`, and any opacity class
           fall-through to the wrapper hides the whole image stack. -->
      <KunImage
        :src="bannerSrc"
        :alt="galgameName"
        aspect-ratio="16 / 9"
        class-name="rounded-t-lg"
      />

      <div
        v-if="showNsfwBadge"
        class="bg-content1 shadow-kun-sm absolute top-2 left-2 z-10 rounded-full"
      >
        <KunChip
          :color="props.patch.content_limit === 'sfw' ? 'success' : 'danger'"
          variant="flat"
        >
          {{ GALGAME_AGE_LIMIT_MAP[props.patch.content_limit] }}
        </KunChip>
      </div>
    </div>

    <div class="flex flex-col justify-between space-y-2 p-3">
      <div class="space-y-0.5">
        <h2
          class="text-medium hover:text-primary-500 space-x-2 font-semibold transition-colors line-clamp-3 sm:text-lg"
        >
          <span>{{ galgameName }}</span>
          <span
            v-if="props.patch.created"
            class="text-default-500 text-xs font-normal"
          >
            {{ formatDistanceToNow(props.patch.created) }}
          </span>
        </h2>
        <p
          v-if="japaneseSubtitle"
          class="text-default-500 line-clamp-1 text-xs"
        >
          {{ japaneseSubtitle }}
        </p>
      </div>
      <div
        v-if="showReleaseDate && releaseDate"
        class="text-default-500 flex items-center gap-1 text-xs"
      >
        <KunIcon name="lucide:calendar" class="size-3.5" />
        <span>{{ releaseDate }} 发售</span>
      </div>
      <KunCardStats :patch="props.patch" />
    </div>

    <div class="px-3 pt-0 pb-3">
      <KunPatchAttribute
        :types="props.patch.type"
        :languages="props.patch.language"
        :platforms="props.patch.platform"
        size="sm"
      />
    </div>
  </KunCard>
</template>
