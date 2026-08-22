<script setup lang="ts">
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const route = useRoute()
const router = useRouter()
const api = useApi()

const goBack = () => router.back()

const characterId = computed(() => Number((route.params as { id: string }).id))

const { data } = await useAsyncData<PatchCharacterDetail | null>(
  () => `galgame-character-${characterId.value}`,
  async () => {
    const res = await api.get<PatchCharacterDetail>(
      `/galgame/character/${characterId.value}`
    )
    return res.code === 0 ? res.data : null
  }
)

const character = computed(() => data.value)

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

useHead(() => ({
  title: name.value ? `${name.value} 的角色资料` : '角色'
}))
</script>

<template>
  <div
    v-if="character"
    class="mx-auto w-full max-w-7xl space-y-6 px-3 py-4"
  >
    <button
      type="button"
      class="text-default-500 hover:text-default-700 flex items-center gap-1 text-sm"
      @click="goBack"
    >
      <KunIcon name="lucide:arrow-left" class="size-4" />
      返回
    </button>

    <div class="bg-content1 shadow-kun-sm rounded-3xl p-6 sm:p-8">
      <div class="flex flex-col gap-5 sm:flex-row">
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
                  class-name="w-32 sm:w-44"
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
                  :class-name="figureSrc ? 'w-20 sm:w-24' : 'w-32 sm:w-44'"
                />
              </button>
            </KunLightboxGalleryItem>
          </div>
        </KunLightboxGallery>

        <div class="min-w-0 grow space-y-3">
          <div class="space-y-1">
            <h1 class="text-2xl font-bold break-words sm:text-3xl">
              {{ name }}
            </h1>
            <p v-if="secondaryName" class="text-default-400 text-sm">
              {{ secondaryName }}
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
    </div>
  </div>

  <KunNull v-else description="未找到该角色" />
</template>
