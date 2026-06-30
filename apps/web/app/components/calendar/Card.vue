<script setup lang="ts">
import { GALGAME_AGE_LIMIT_MAP } from '~/constants/galgame'

interface Props {
  item: CalendarItem
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
</script>

<template>
  <!-- Every entry links to /patch/:id. moyu has a patch → the normal detail page
       (badged 本站有补丁); no patch yet → the read-only "本站暂无补丁" galgame page
       (the backend lazily renders wiki metadata + a 发布补丁 CTA). -->
  <NuxtLink
    :to="`/patch/${props.item.id}/introduction`"
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
    </div>
  </NuxtLink>
</template>
