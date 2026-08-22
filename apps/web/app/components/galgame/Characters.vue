<script setup lang="ts">
import {
  GALGAME_CHARACTER_KIND_COLOR,
  GALGAME_CHARACTER_KIND_MAP,
  GALGAME_CHARACTER_SPOILER_MAP
} from '~/constants/galgameEntity'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  characters: PatchDetailCharacter[]
}>()

const COLLAPSED = 12

const isSpoilerRevealed = ref(false)
const isExpanded = ref(false)

const spoilerCount = computed(
  () => props.characters.filter((c) => c.spoiler > 0).length
)

const visible = computed(() =>
  isSpoilerRevealed.value
    ? props.characters
    : props.characters.filter((c) => c.spoiler === 0)
)

// The head shot is the grid's default because every character that has standing
// art also has one, so preferring it keeps all the cards on one crop. `figure`
// only appears for a character catalog has art for but no portrait of.
const artOf = (c: PatchDetailCharacter) => c.image_hash || c.figure_hash || ''

const withArt = computed(() => visible.value.filter((c) => !!artOf(c)))
const nameOnly = computed(() => visible.value.filter((c) => !artOf(c)))

const isCollapsible = computed(() => withArt.value.length > COLLAPSED)
const visibleArt = computed(() =>
  isCollapsible.value && !isExpanded.value
    ? withArt.value.slice(0, COLLAPSED)
    : withArt.value
)

const thumbOf = (c: PatchDetailCharacter) => imageServiceUrl(artOf(c), 'mini')

// A head shot fills the box; standing art is letterboxed rather than cropped
// through the character.
const fitOf = (c: PatchDetailCharacter) =>
  c.image_hash ? ('cover' as const) : ('contain' as const)

const nameOf = (c: PatchDetailCharacter) => getPreferredLanguageText(c.name)
const secondaryOf = (c: PatchDetailCharacter) =>
  getSecondaryLanguageText(c.name, nameOf(c))

const voicesOf = (c: PatchDetailCharacter) =>
  c.voices.map((v) => getPreferredLanguageText(v.name)).join(' / ')
</script>

<template>
  <section v-if="characters.length" class="space-y-4">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <KunHeader
        name="登场角色"
        description="该 Galgame 的登场角色与配音演员, 资料来自 鲲 Galgame 目录"
        scale="h2"
      />
      <KunButton
        v-if="spoilerCount"
        variant="flat"
        color="warning"
        size="sm"
        @click="isSpoilerRevealed = !isSpoilerRevealed"
      >
        <KunIcon :name="isSpoilerRevealed ? 'lucide:eye-off' : 'lucide:eye'" />
        {{
          isSpoilerRevealed
            ? `隐藏 ${spoilerCount} 名剧透角色`
            : `显示 ${spoilerCount} 名剧透角色`
        }}
      </KunButton>
    </div>

    <div
      v-if="visibleArt.length"
      class="grid grid-cols-3 gap-3 sm:grid-cols-[repeat(auto-fill,minmax(128px,1fr))]"
    >
      <NuxtLink
        v-for="c in visibleArt"
        :key="c.id"
        :to="`/galgame/character/${c.id}`"
        class="group space-y-1.5 text-left"
        :aria-label="`查看角色 ${nameOf(c)}`"
      >
        <div
          class="bg-default-100 group-hover:ring-primary group-focus:ring-primary relative overflow-hidden rounded-lg ring-1 ring-transparent transition-all"
        >
          <KunImage
            :src="thumbOf(c)"
            :alt="nameOf(c)"
            loading="lazy"
            aspect-ratio="3/4"
            :object-fit="fitOf(c)"
            class-name="w-full"
            image-class-name="transition-transform duration-200 group-hover:scale-105"
          />
          <KunChip
            v-if="GALGAME_CHARACTER_KIND_MAP[c.kind]"
            :color="GALGAME_CHARACTER_KIND_COLOR[c.kind] ?? 'default'"
            size="xs"
            class-name="absolute top-1 left-1"
          >
            {{ GALGAME_CHARACTER_KIND_MAP[c.kind] }}
          </KunChip>
        </div>

        <div class="space-y-0.5">
          <p class="text-default-800 truncate text-sm font-medium">
            {{ nameOf(c) }}
          </p>
          <p
            v-if="secondaryOf(c)"
            class="text-default-400 truncate text-xs"
            :title="secondaryOf(c)"
          >
            {{ secondaryOf(c) }}
          </p>
          <p
            v-if="c.voices.length"
            class="text-default-500 truncate text-xs"
            :title="voicesOf(c)"
          >
            CV {{ voicesOf(c) }}
          </p>
          <p
            v-if="GALGAME_CHARACTER_SPOILER_MAP[c.spoiler]"
            class="text-warning text-xs"
          >
            {{ GALGAME_CHARACTER_SPOILER_MAP[c.spoiler] }}
          </p>
        </div>
      </NuxtLink>
    </div>

    <KunButton
      v-if="isCollapsible"
      variant="flat"
      color="primary"
      size="sm"
      @click="isExpanded = !isExpanded"
    >
      <KunIcon
        :name="isExpanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
      />
      {{
        isExpanded
          ? '收起角色'
          : `展开其余 ${withArt.length - COLLAPSED} 名角色`
      }}
    </KunButton>

    <div v-if="nameOnly.length" class="space-y-1.5">
      <p v-if="visibleArt.length" class="text-default-500 text-sm">
        其他登场角色
      </p>
      <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1.5">
        <span v-for="c in nameOnly" :key="c.id" class="text-sm">
          <NuxtLink
            :to="`/galgame/character/${c.id}`"
            class="text-default-800 hover:text-primary"
          >
            {{ nameOf(c) }}
          </NuxtLink>
          <span v-if="c.voices.length" class="text-default-400">
            （CV {{ voicesOf(c) }}）
          </span>
        </span>
      </div>
    </div>
  </section>
</template>
