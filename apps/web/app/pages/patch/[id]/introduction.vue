<script setup lang="ts">
import { useContentBlurUp } from '@kungal/ui-vue'
import { GALGAME_OFFICIAL_CATEGORY_MAP } from '~/constants/galgameEntity'
import { kunMoyuMoe } from '~/config/moyu-moe'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const route = useRoute()
const api = useApi()
const settingStore = useSettingStore()

const galgameId = computed(() => Number(route.params.id))

const { data: detail } = await useAsyncData<PatchDetail | null>(
  () => `patch-detail-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchDetail>(`/patch/${galgameId.value}/detail`)
    return res.code === 0 ? res.data : null
  }
)

const lang = ref<Language>('zh-cn')

const introEl = ref<HTMLElement | null>(null)
useContentBlurUp(introEl)

const pickInitialLang = () => {
  if (!detail.value?.introduction_html) return 'zh-cn' as Language
  const langs: Language[] = ['zh-cn', 'ja-jp', 'en-us']
  return (
    langs.find((l) => detail.value!.introduction_html[l]) ??
    ('zh-cn' as Language)
  )
}

watchEffect(() => {
  lang.value = pickInitialLang()
})

const introHtml = computed(() => {
  if (!detail.value?.introduction_html) return ''
  return getPreferredLanguageText(detail.value.introduction_html, lang.value)
})

const langOptions: { value: Language; label: string }[] = [
  { value: 'zh-cn', label: '中文' },
  { value: 'ja-jp', label: '日本語' },
  { value: 'en-us', label: 'English' }
]

type TagCategory = 'content' | 'sexual'
const CATEGORY_LABEL: Record<TagCategory, string> = {
  content: '内容',
  sexual: '性相关'
}
const tagColor = (cat: string): 'primary' | 'danger' => {
  if (cat === 'sexual') return 'danger'
  return 'primary'
}
const TAG_CATEGORY_TEXT_CLASS: Record<TagCategory, string> = {
  content: 'text-primary-600',
  sexual: 'text-danger-600'
}

const isSafeMode = computed(() => settingStore.data.kunNsfwEnable === 'sfw')
const availableCategories = computed<TagCategory[]>(() =>
  isSafeMode.value ? ['content'] : ['content', 'sexual']
)

type SpoilerMode = 'none' | 'minor' | 'all'
const spoilerMode = ref<SpoilerMode>('none')
const spoilerThreshold = computed(() =>
  spoilerMode.value === 'all' ? 2 : spoilerMode.value === 'minor' ? 1 : 0
)
const spoilerOptions = [
  { value: 'none', label: '隐藏剧透' },
  { value: 'minor', label: '轻微剧透' },
  { value: 'all', label: '完全剧透' }
]

const visibleCategories = ref<Set<TagCategory>>(new Set(['content', 'sexual']))
const toggleCategory = (c: TagCategory) => {
  if (visibleCategories.value.has(c)) visibleCategories.value.delete(c)
  else visibleCategories.value.add(c)
  visibleCategories.value = new Set(visibleCategories.value)
}

const filteredTags = computed(() => {
  if (!detail.value?.tags) return []
  return detail.value.tags.filter((t) => {
    const cat = (t.category || 'content') as TagCategory
    if (isSafeMode.value && cat === 'sexual') return false
    if ((t.spoiler_level ?? 0) > spoilerThreshold.value) return false
    if (!visibleCategories.value.has(cat)) return false
    return true
  })
})

const hiddenByFilterCount = computed(() => {
  if (!detail.value?.tags) return 0
  return detail.value.tags.length - filteredTags.value.length
})

// 声优 repeats the CV line every character card above already prints, and with
// 40 entries on a well-credited work it buries 剧本 / 原画 / 音乐 — the roles a
// reader opens this section for. Drop it only when the roster actually showed
// them; a work with credits but no roster still needs the list.
const staffGroups = computed(() => {
  const groups = detail.value?.staff ?? []
  const rosterNamesVoices = (detail.value?.characters ?? []).some(
    (c) => c.voices.length > 0
  )
  return rosterNamesVoices
    ? groups.filter((g) => g.role_key !== 'voice-actor')
    : groups
})

const officialLogoSrc = (o: PatchDetailOfficial) =>
  imageServiceUrl((o.logo_hash ?? '').trim(), 'mini')

const officialCategory = (o: PatchDetailOfficial) =>
  GALGAME_OFFICIAL_CATEGORY_MAP[o.category] ?? ''

const kungalOrigin = kunMoyuMoe.domain.kungal
</script>

<template>
  <div v-if="detail" class="space-y-10">
    <section class="space-y-4">
      <KunHeader name="简介" scale="h2">
        <template #headerEndContent>
          <KunSelect
            :model-value="lang"
            :options="langOptions"
            class-name="max-w-36"
            @update:model-value="
              (v: Language | Language[] | null) => (lang = v as Language)
            "
          />
        </template>
      </KunHeader>

      <div
        v-if="introHtml"
        ref="introEl"
        class="kun-prose max-w-none"
        v-html="introHtml"
      />
      <KunNull v-else description="此 Galgame 暂无简介，可到 鲲 Galgame 补充" />

      <div class="text-default-500 grid gap-4 sm:grid-cols-2">
        <div class="flex items-center gap-2 text-sm">
          <KunIcon name="lucide:clock" class="size-4" />
          <span>
            创建时间: {{ formatDate(detail.created, { isShowYear: true }) }}
          </span>
        </div>
        <div class="flex items-center gap-2 text-sm">
          <KunIcon name="lucide:refresh-cw" class="size-4" />
          <span>
            更新时间: {{ formatDate(detail.updated, { isShowYear: true }) }}
          </span>
        </div>
        <div v-if="detail.vndb_id" class="flex items-center gap-2 text-sm">
          <KunIcon name="lucide:link" class="size-4" />
          <span>
            VNDB ID:
            <a
              :href="`https://vndb.org/${detail.vndb_id}`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              {{ detail.vndb_id }}
            </a>
          </span>
        </div>
        <div v-if="detail.galgame" class="flex items-center gap-2 text-sm">
          <KunIcon name="lucide:book-open" class="size-4" />
          <span>
            鲲 Galgame:
            <a
              :href="`${kungalOrigin}/galgame/${detail.galgame.id}`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              #{{ detail.galgame.id }}（完整资料 / 修订历史）
            </a>
          </span>
        </div>
        <div v-if="detail.bid" class="flex items-center gap-2 text-sm">
          <KunIcon name="lucide:tv" class="size-4" />
          <span>
            Bangumi ID:
            <a
              :href="`https://bangumi.tv/subject/${detail.bid}`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              {{ detail.bid }}
            </a>
          </span>
        </div>
      </div>
    </section>

    <GalgameRatings
      :ratings="detail.ratings ?? []"
      :vndb-id="detail.vndb_id"
      :bid="detail.bid"
    />

    <section v-if="detail.tags?.length" class="space-y-4">
      <KunHeader name="标签" scale="h2">
        <template #headerEndContent>
          <div class="flex flex-wrap items-center gap-x-6 gap-y-3 text-sm">
            <div class="flex items-center gap-2">
              <span class="text-default-500 shrink-0">剧透</span>
              <KunRadioGroup
                v-model="spoilerMode"
                orientation="horizontal"
                :options="spoilerOptions"
              />
            </div>
            <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
              <span class="text-default-500 shrink-0">分类</span>
              <KunCheckBox
                v-for="c in availableCategories"
                :key="c"
                :model-value="visibleCategories.has(c)"
                color="primary"
                @change="toggleCategory(c)"
              >
                <span :class="TAG_CATEGORY_TEXT_CLASS[c]">
                  {{ CATEGORY_LABEL[c] }}
                </span>
              </KunCheckBox>
            </div>
          </div>
        </template>
      </KunHeader>

      <div class="flex flex-wrap gap-2">
        <NuxtLink
          v-for="t in filteredTags"
          :key="t.id"
          :to="`/galgame/tag/${t.id}`"
        >
          <KunChip :color="tagColor(t.category)" variant="flat" size="sm">
            <KunIcon
              v-if="t.spoiler_level > 0"
              name="lucide:eye-off"
              class="mr-0.5 size-3.5"
            />
            {{ t.name }}
          </KunChip>
        </NuxtLink>
        <span
          v-if="!filteredTags.length"
          class="text-default-400 text-sm italic"
        >
          (当前筛选条件下没有标签可显示)
        </span>
        <span
          v-else-if="hiddenByFilterCount > 0"
          class="text-default-400 self-center text-xs"
        >
          已隐藏 {{ hiddenByFilterCount }} 个
        </span>
      </div>
    </section>

    <section v-if="detail.officials?.length" class="space-y-4">
      <KunHeader name="会社" scale="h2" />
      <div class="flex flex-wrap gap-2">
        <NuxtLink
          v-for="o in detail.officials"
          :key="o.id"
          :to="`/galgame/official/${o.id}`"
        >
          <KunChip color="success" variant="flat" size="sm">
            <KunImage
              v-if="officialLogoSrc(o)"
              :src="officialLogoSrc(o)"
              :alt="o.name"
              object-fit="contain"
              class-name="mr-1 size-4 shrink-0 rounded-sm"
            />
            {{ o.name }}
            <span v-if="officialCategory(o)" class="text-default-500 text-xs">
              · {{ officialCategory(o) }}
            </span>
          </KunChip>
        </NuxtLink>
      </div>
    </section>

    <GalgameCharacters :characters="detail.characters ?? []" />

    <GalgameStaff :staff="staffGroups" />

    <GalgameSeries :series="detail.series ?? []" />

    <section v-if="detail.galgame?.screenshots?.length">
      <GalgameGallery :screenshots="detail.galgame.screenshots" />
    </section>

    <section v-if="detail.galgame">
      <p class="text-default-500 text-sm">
        游戏资料、修订历史等更多信息请查看
        <a
          :href="`${kungalOrigin}/galgame/${detail.galgame.id}`"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary hover:underline"
        >
          鲲 Galgame
        </a>
        。
      </p>
    </section>
  </div>

  <KunNull v-else description="加载失败" />
</template>
