<script setup lang="ts">
const props = defineProps<{ relations: PatchDetailRelated[] }>()

const CARD_LIMIT = 12
const OTHER_LIMIT = 6
const MEDIUM_LABEL: Record<string, string> = {
  anime: '动画',
  manga: '漫画',
  novel: '小说',
  asmr: '音声',
  doujin_game: '同人游戏',
  music: '音乐'
}

const games = computed(() =>
  props.relations.filter((r) => r.medium === 'galgame')
)
const others = computed(() =>
  props.relations.filter((r) => r.medium !== 'galgame')
)

const gamesOpen = ref(false)
const othersOpen = ref(false)

const shownGames = computed(() =>
  gamesOpen.value ? games.value : games.value.slice(0, CARD_LIMIT)
)
const shownOthers = computed(() =>
  othersOpen.value ? others.value : others.value.slice(0, OTHER_LIMIT)
)

const nameOf = (r: PatchDetailRelated) => getPreferredLanguageText(r.name)
</script>

<template>
  <section v-if="relations.length" class="space-y-4">
    <KunHeader name="关联作品" scale="h2" />

    <GalgameCardGrid v-if="games.length">
      <div v-for="r in shownGames" :key="r.id" class="flex h-full flex-col">
        <KunNsfwMask :nsfw="r.content_limit !== 'sfw'" rounded="rounded-lg">
          <NuxtLink
            :to="`/galgame/${r.id}`"
            :aria-label="nameOf(r)"
            tabindex="-1"
            class="relative block overflow-hidden rounded-lg"
          >
            <KunImage
              v-if="resolvePortraitUrl(r)"
              :src="resolvePortraitUrl(r)"
              :alt="nameOf(r)"
              aspect-ratio="5 / 7"
              :thumbhash="resolvePortraitThumbhash(r)"
              class-name="transition-transform duration-300 hover:scale-105"
            />
            <div
              v-else
              class="bg-default-100 text-default-400 flex items-center justify-center"
              style="aspect-ratio: 5 / 7"
            >
              <KunIcon name="lucide:image-off" class="size-6" />
            </div>
          </NuxtLink>
        </KunNsfwMask>

        <div class="pt-1.5">
          <h3 class="line-clamp-2 text-sm font-medium">
            <NuxtLink
              :to="`/galgame/${r.id}`"
              class="hover:text-primary-500 transition-colors"
            >
              {{ nameOf(r) }}
            </NuxtLink>
          </h3>
          <p class="text-default-500 mt-0.5 text-xs">
            {{ r.phrases.join(' · ') }}
          </p>
        </div>
      </div>
    </GalgameCardGrid>

    <div v-if="games.length > CARD_LIMIT" class="flex justify-center">
      <KunButton
        variant="flat"
        color="primary"
        size="sm"
        @click="gamesOpen = !gamesOpen"
      >
        <KunIcon
          :name="gamesOpen ? 'lucide:chevron-up' : 'lucide:chevron-down'"
        />
        {{ gamesOpen ? '收起' : `展开全部（${games.length}）` }}
      </KunButton>
    </div>

    <p v-if="others.length" class="text-default-600 text-sm leading-relaxed">
      <span class="text-default-500">其他媒介：</span>
      <template v-for="(r, i) in shownOthers" :key="r.id">
        <span v-if="i" class="text-default-400">· </span>
        <span>{{ nameOf(r) }}（{{ MEDIUM_LABEL[r.medium] ?? r.medium }}）</span>
      </template>
      <KunButton
        v-if="others.length > OTHER_LIMIT"
        size="xs"
        variant="light"
        color="primary"
        class-name="ml-1"
        @click="othersOpen = !othersOpen"
      >
        {{ othersOpen ? '收起' : `展开全部（${others.length}）` }}
      </KunButton>
    </p>
  </section>
</template>
