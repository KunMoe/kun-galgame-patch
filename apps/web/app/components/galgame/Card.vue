<script setup lang="ts">
import { GALGAME_AGE_LIMIT_MAP } from '~/constants/galgame'

interface Props {
  patch: GalgameCard
}

const props = defineProps<Props>()

const galgameName = computed(() => getPreferredLanguageText(props.patch.name))

const bannerSrc = computed(
  () => resolveBannerUrl(props.patch, 'mini') || '/kungalgame-trans.webp'
)
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

      <div class="bg-background absolute top-2 left-2 z-10 rounded-full">
        <KunChip
          :color="props.patch.content_limit === 'sfw' ? 'success' : 'danger'"
          variant="flat"
        >
          {{ GALGAME_AGE_LIMIT_MAP[props.patch.content_limit] }}
        </KunChip>
      </div>
    </div>

    <div class="flex flex-col justify-between space-y-2 p-3">
      <h2
        class="text-medium hover:text-primary-500 space-x-2 font-semibold transition-colors line-clamp-2 sm:text-lg"
      >
        <span>{{ galgameName }}</span>
        <span
          v-if="props.patch.created"
          class="text-default-500 text-xs font-normal"
        >
          {{ formatDistanceToNow(props.patch.created) }}
        </span>
      </h2>
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
