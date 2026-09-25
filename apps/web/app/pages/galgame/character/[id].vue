<script setup lang="ts">
import {
  GALGAME_CHARACTER_KIND_MAP,
  GALGAME_CHARACTER_SPOILER_MAP,
  GALGAME_STAFF_GENDER_MAP
} from '~/constants/galgameEntity'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

defineOptions({ name: 'character-detail' })

const route = useRoute()
const api = useApi()

const characterID = computed(() => Number(route.params.id))

const VERDICT_NOT_FOUND = 40400

const { data, pending } = await useAsyncData(
  () => `character-detail-${characterID.value}`,
  async () => {
    const res = await api.get<PatchCharacterDetail>(
      `/galgame/character/${characterID.value}`
    )
    return { code: res.code, detail: res.code === 0 ? res.data : null }
  },
  { watch: [characterID] }
)

const notFound = () => kunPageError(404, '角色不存在')

if (data.value?.code === VERDICT_NOT_FOUND) throw notFound()
watch(data, (v) => {
  if (v?.code === VERDICT_NOT_FOUND) showError(notFound())
})

const character = computed(() => data.value?.detail ?? null)

const works = useEntityWorks(
  () => `/galgame/character/${characterID.value}/works`,
  character
)

const name = computed(() => getPreferredLanguageText(character.value?.name))
const secondaryName = computed(() =>
  getSecondaryLanguageText(character.value?.name, name.value)
)

const figureSrc = computed(() =>
  imageServiceUrl(character.value?.figure_hash ?? '')
)
const imageSrc = computed(() =>
  imageServiceUrl(character.value?.image_hash ?? '')
)

const intro = computed(() => pickPreferredLanguageRow(character.value?.intros))

const aliases = computed(() =>
  (character.value?.aliases ?? []).filter(
    (alias) => alias !== name.value && alias !== secondaryName.value
  )
)

const isTraitSpoilerRevealed = ref(false)
const traits = computed(() =>
  (character.value?.traits ?? []).filter(
    (t) => isTraitSpoilerRevealed.value || t.spoiler === 0
  )
)
const hiddenTraitCount = computed(
  () => (character.value?.traits ?? []).filter((t) => t.spoiler > 0).length
)

// Catalog sends the traits already grouped and in order, so consecutive rows
// sharing a group name are one group; do not sort, that scatters them.
const traitGroups = computed(() => {
  const groups: { name: string; traits: PatchCharacterTrait[] }[] = []
  for (const trait of traits.value) {
    const group = trait.group || '其他'
    const last = groups.at(-1)
    if (last?.name === group) {
      last.traits.push(trait)
    } else {
      groups.push({ name: group, traits: [trait] })
    }
  }
  return groups
})

const facts = computed(() => {
  const c = character.value
  if (!c) return []
  const rows: string[] = []
  const gender = c.gender
  if (gender && GALGAME_STAFF_GENDER_MAP[gender]) {
    rows.push(GALGAME_STAFF_GENDER_MAP[gender])
  }
  if (c.birth_m && c.birth_d) rows.push(`生日 ${c.birth_m} 月 ${c.birth_d} 日`)
  return rows
})

const voicesOf = (work: PatchEntityWork) =>
  (work.voices ?? []).map((v) => getPreferredLanguageText(v.name)).join(' / ')

if (character.value) {
  useKunSeoMeta({
    title: `角色 · ${name.value}`,
    description: `Galgame 角色 ${name.value} 的资料、特征与登场作品`
  })
} else {
  useKunDisableSeo('角色资料')
}
</script>

<template>
  <div class="container mx-auto my-6">
    <KunLoading v-if="pending && !character" description="加载中..." />

    <KunNull v-else-if="!character" description="角色加载失败，请稍后重试" />

    <template v-else>
      <div class="flex flex-col gap-6 sm:flex-row">
        <KunLightboxGallery v-if="figureSrc || imageSrc">
          <div class="flex shrink-0 items-start gap-3 sm:flex-col">
            <KunLightboxGalleryItem
              v-if="figureSrc"
              v-slot="{ open }"
              :src="figureSrc"
              :alt="name"
              :wrap="false"
            >
              <button
                type="button"
                class="bg-default-100 w-fit cursor-zoom-in overflow-hidden rounded-xl"
                :aria-label="`查看 ${name} 的立绘`"
                @click="open"
              >
                <KunImage
                  :src="figureSrc"
                  :alt="name"
                  loading="eager"
                  object-fit="contain"
                  class-name="w-32 sm:w-52"
                />
              </button>
            </KunLightboxGalleryItem>

            <KunLightboxGalleryItem
              v-if="imageSrc"
              v-slot="{ open }"
              :src="imageSrc"
              :alt="name"
              :wrap="false"
            >
              <button
                type="button"
                class="bg-default-100 w-fit cursor-zoom-in overflow-hidden rounded-xl"
                :aria-label="`查看 ${name} 的头像`"
                @click="open"
              >
                <KunImage
                  :src="imageSrc"
                  :alt="name"
                  loading="eager"
                  aspect-ratio="3/4"
                  object-fit="cover"
                  :class-name="figureSrc ? 'w-20 sm:w-28' : 'w-32 sm:w-52'"
                />
              </button>
            </KunLightboxGalleryItem>
          </div>
        </KunLightboxGallery>

        <div class="min-w-0 grow space-y-4">
          <div class="space-y-1">
            <h1 class="text-2xl font-bold break-words sm:text-3xl">
              {{ name }}
            </h1>
            <p v-if="secondaryName" class="text-default-400 text-sm">
              {{ secondaryName }}
            </p>
            <p v-if="facts.length" class="text-default-500 text-sm">
              {{ facts.join(' · ') }}
            </p>
            <p v-if="aliases.length" class="text-default-500 text-sm">
              别名 {{ aliases.join(' / ') }}
            </p>
          </div>

          <div v-if="intro" class="space-y-1">
            <p class="text-default-600 text-sm whitespace-pre-line">
              {{ intro.intro }}
            </p>
            <p class="text-default-400 text-xs">
              资料来自 {{ intro.source || '鲲 Galgame 目录'
              }}<template v-if="intro.machine">, 由机器翻译</template>
            </p>
          </div>

          <div v-if="traitGroups.length" class="space-y-2">
            <div
              v-for="group in traitGroups"
              :key="group.name"
              class="space-y-1"
            >
              <p class="text-default-400 text-xs">{{ group.name }}</p>
              <div class="flex flex-wrap gap-1.5">
                <KunChip
                  v-for="trait in group.traits"
                  :key="trait.id"
                  size="xs"
                  :color="trait.spoiler > 0 ? 'warning' : 'default'"
                >
                  {{ trait.name }}<template v-if="trait.lie">（伪）</template>
                </KunChip>
              </div>
            </div>
          </div>

          <KunButton
            v-if="hiddenTraitCount && !isTraitSpoilerRevealed"
            variant="flat"
            color="warning"
            size="sm"
            @click="isTraitSpoilerRevealed = true"
          >
            <KunIcon name="lucide:eye" />
            显示 {{ hiddenTraitCount }} 条剧透特征
          </KunButton>

          <div
            v-if="character.links.length"
            class="flex flex-wrap items-center gap-x-4 gap-y-2"
          >
            <a
              v-for="link in character.links"
              :key="link.url"
              :href="link.url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-default-500 hover:text-primary text-sm"
            >
              {{ link.name }}
              <KunIcon name="lucide:external-link" class="inline size-3" />
            </a>
          </div>
        </div>
      </div>

      <section class="mt-8 space-y-4">
        <KunHeader
          name="登场作品"
          description="该角色登场的 Galgame, 资料来自 鲲 Galgame 目录"
          scale="h2"
        />

        <KunNull
          v-if="!works.items.value.length"
          description="暂无该角色登场的 Galgame"
        />

        <GalgameList v-else :items="works.items.value">
          <template #meta="{ item }">
            <div class="mt-1 space-y-0.5 text-xs">
              <p v-if="item.voices?.length" class="text-default-500 truncate">
                CV {{ voicesOf(item) }}
              </p>
              <p v-if="item.spoiler" class="text-warning">
                {{ GALGAME_CHARACTER_SPOILER_MAP[item.spoiler] }}
              </p>
              <p
                v-else-if="
                  item.roster_role &&
                  GALGAME_CHARACTER_KIND_MAP[item.roster_role]
                "
                class="text-default-400"
              >
                {{ GALGAME_CHARACTER_KIND_MAP[item.roster_role] }}
              </p>
            </div>
          </template>
        </GalgameList>

        <div v-if="works.hasMore.value" class="flex justify-center">
          <KunButton
            variant="flat"
            color="primary"
            :is-loading="works.isLoading.value"
            @click="works.loadMore"
          >
            加载更多作品
          </KunButton>
        </div>
      </section>
    </template>
  </div>
</template>
