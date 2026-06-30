<script setup lang="ts">
import { GALGAME_AGE_LIMIT_MAP } from '~/constants/galgame'

interface Props {
  item: CalendarItem
  // Wiki frontend origin, passed down so each card doesn't re-read runtimeConfig.
  wikiOrigin: string
}

const props = defineProps<Props>()

// Title language follows the same /galgame "显示设置" cookie as GalgameCard.
const settingStore = useSettingStore()
const titleLanguage = computed(() => settingStore.data.titleLanguage ?? 'ja-jp')
const name = computed(() =>
  getPreferredLanguageText(props.item.name, titleLanguage.value)
)

const bannerSrc = computed(
  () => resolveBannerUrl(props.item, 'mini') || '/kungalgame-trans.webp'
)

// Smart link: galgames moyu has a patch for go to the local 补丁页; the rest go
// to the wiki entry page (new tab, since it leaves moyu). `<component :is>` keeps
// the internal route a real NuxtLink (SPA nav / prefetch) and the external one a
// plain target=_blank anchor.
const NuxtLink = resolveComponent('NuxtLink')
const isInternal = computed(() => props.item.has_patch)
const link = computed(() =>
  isInternal.value
    ? `/patch/${props.item.id}/introduction`
    : `${props.wikiOrigin}/galgame/${props.item.id}`
)
</script>

<template>
  <component
    :is="isInternal ? NuxtLink : 'a'"
    :to="isInternal ? link : undefined"
    :href="isInternal ? undefined : link"
    :target="isInternal ? undefined : '_blank'"
    :rel="isInternal ? undefined : 'noopener noreferrer'"
    class="group border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block overflow-hidden rounded-lg border transition-colors"
  >
    <div class="relative">
      <KunImage
        :src="bannerSrc"
        :alt="name"
        aspect-ratio="16 / 9"
        :thumbhash="resolveBannerThumbhash(item)"
        class-name="w-full"
      />
      <div class="absolute top-1.5 left-1.5">
        <KunChip
          :color="item.content_limit === 'sfw' ? 'success' : 'danger'"
          variant="flat"
          size="sm"
        >
          {{ GALGAME_AGE_LIMIT_MAP[item.content_limit] }}
        </KunChip>
      </div>
      <div v-if="item.has_patch" class="absolute top-1.5 right-1.5">
        <KunChip color="primary" variant="solid" size="sm">本站有补丁</KunChip>
      </div>
    </div>

    <div class="p-2.5">
      <h3
        class="group-hover:text-primary-500 text-sm font-medium transition-colors line-clamp-2"
      >
        {{ name }}
      </h3>
      <p
        v-if="!item.has_patch"
        class="text-default-400 mt-1 flex items-center gap-1 text-xs"
      >
        <KunIcon name="lucide:external-link" class="size-3 shrink-0" />
        前往词条查看
      </p>
    </div>
  </component>
</template>
