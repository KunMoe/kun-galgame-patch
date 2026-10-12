<script setup lang="ts">
import {
  SEARCH_ENTITY_FAMILY_MAP,
  searchEntityPath,
  searchEntityWorkCount
} from './items'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  item: SearchEntityItem
  keywords?: string
}>()

const meta = computed(() => SEARCH_ENTITY_FAMILY_MAP[props.item.family])

const name = computed(() => getPreferredLanguageText(props.item.name))
const secondaryName = computed(() =>
  getSecondaryLanguageText(props.item.name, name.value)
)

const src = computed(() => imageServiceUrl(props.item.image_hash ?? ''))

// A character's picture is a portrait, so it is cropped to the top of the frame
// — centring it on a 220x320 bust cuts the face off. A logo is drawn whole.
const isPortrait = computed(() => props.item.family === 'character')

const workCount = computed(() => searchEntityWorkCount(props.item))
</script>

<template>
  <KunCard :href="searchEntityPath(item)" :is-hoverable="true" padding="none">
    <div class="flex w-full items-center gap-3 p-2">
      <KunImage
        v-if="src"
        :src="src"
        :alt="name"
        loading="lazy"
        :object-fit="isPortrait ? 'cover' : 'contain'"
        class-name="bg-default-100 size-14 shrink-0 overflow-hidden rounded-lg"
        :image-class-name="cn('size-full', isPortrait ? 'object-top' : 'p-1.5')"
      />
      <span
        v-else
        class="bg-default-100 text-default-500 flex size-14 shrink-0 items-center justify-center rounded-lg"
      >
        <KunIcon :name="meta.icon" class="size-6" />
      </span>

      <span class="min-w-0 flex-1 space-y-0.5">
        <span class="block truncate text-sm font-medium">
          <SearchHighlight :text="name" :keywords="keywords" />
        </span>
        <span
          v-if="secondaryName"
          class="text-default-500 block truncate text-xs"
        >
          {{ secondaryName }}
        </span>
        <span
          v-if="workCount"
          class="text-default-400 block text-xs tabular-nums"
        >
          {{ workCount }}
        </span>
      </span>
    </div>
  </KunCard>
</template>
