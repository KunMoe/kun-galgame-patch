<script setup lang="ts">
import { GALGAME_AGE_LIMIT_MAP } from '~/constants/galgame'

interface Props {
  patch: GalgameCard
}

const props = defineProps<Props>()

const galgameName = computed(() => getPreferredLanguageText(props.patch.name))

const imageLoaded = ref(false)

const bannerSrc = computed(
  () => resolveBannerUrl(props.patch, 'mini') || '/kungalgame-trans.webp'
)
</script>

<template>
  <KunCard
    :href="`/patch/${props.patch.id}/introduction`"
    class-name="w-full"
    content-class="gap-0 p-0"
  >
    <div
      class="relative mx-auto w-full overflow-hidden rounded-t-lg text-center opacity-90"
    >
      <div
        :class="
          cn(
            'bg-default-100 absolute inset-0 animate-pulse',
            imageLoaded ? 'opacity-0' : 'opacity-90',
            'transition-opacity duration-300'
          )
        "
        style="aspect-ratio: 16 / 9"
      />
      <KunImage
        :alt="galgameName"
        :src="bannerSrc"
        :class="
          cn(
            'size-full object-cover transition-all duration-300',
            imageLoaded ? 'scale-100 opacity-90' : 'scale-105 opacity-0'
          )
        "
        style="aspect-ratio: 16 / 9"
        @load="imageLoaded = true"
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
