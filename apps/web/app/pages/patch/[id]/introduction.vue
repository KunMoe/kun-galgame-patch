<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const route = useRoute()
const api = useApi()

const galgameId = computed(() => Number(route.params.id))

const { data: detail } = await useAsyncData<PatchDetail | null>(
  () => `patch-detail-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchDetail>(`/patch/${galgameId.value}/detail`)
    return res.code === 0 ? res.data : null
  }
)

const lang = ref<Language>('zh-cn')

const pickInitialLang = () => {
  if (!detail.value?.introduction_html) return 'zh-cn' as Language
  const langs: Language[] = ['zh-cn', 'ja-jp', 'en-us']
  return (
    langs.find((l) => detail.value!.introduction_html[l]) ?? ('zh-cn' as Language)
  )
}

watchEffect(() => {
  lang.value = pickInitialLang()
})

// introduction_html is pre-rendered server-side via the markdown package; we
// only need to sanitize before mounting it. ADD_ATTR keeps mention links'
// data-id attribute (the renderer adds class="kun-mention").
const introHtml = computed(() => {
  if (!detail.value?.introduction_html) return ''
  const text = getPreferredLanguageText(
    detail.value.introduction_html,
    lang.value
  )
  return DOMPurify.sanitize(text, { ADD_ATTR: ['data-id'] })
})

const langOptions = [
  { value: 'zh-cn', label: '中文' },
  { value: 'ja-jp', label: '日本語' },
  { value: 'en-us', label: 'English' }
]

// ─── Tag display: color-by-category + manual spoiler / category filter ──
// Mirrors the legacy nextjs project (apps/next-web/components/patch/intro
// duction/section/Tag.tsx) — content=primary, sexual=danger, technical=
// success, plus a "显示剧透" checkbox for spoiler_level>0 tags.
// Removed the VNDB/Bangumi provider tab from the legacy impl: Wiki has no
// provider concept (the old shape was bgm vs vndb pre-D8/D11; current Wiki
// taxonomy is a single source).
type TagCategory = 'content' | 'sexual' | 'technical'
const CATEGORY_LABEL: Record<TagCategory, string> = {
  content: '内容',
  sexual: '性相关',
  technical: '技术'
}
const tagColor = (cat: string): 'primary' | 'danger' | 'success' => {
  if (cat === 'sexual') return 'danger'
  if (cat === 'technical') return 'success'
  return 'primary' // content + unknown fallback
}
// Static class map — Tailwind JIT only sees literal class strings, so we can
// NEVER write `text-${color}-600` (the spec calls this out as a latent bug).
const TAG_CATEGORY_TEXT_CLASS: Record<TagCategory, string> = {
  content: 'text-primary-600',
  sexual: 'text-danger-600',
  technical: 'text-success-600'
}

const showSpoiler = ref(false)
const visibleCategories = ref<Set<TagCategory>>(
  new Set(['content', 'sexual', 'technical'])
)
const toggleCategory = (c: TagCategory) => {
  if (visibleCategories.value.has(c)) visibleCategories.value.delete(c)
  else visibleCategories.value.add(c)
  // trigger reactivity by re-assigning the Set
  visibleCategories.value = new Set(visibleCategories.value)
}

const filteredTags = computed(() => {
  if (!detail.value?.tags) return []
  return detail.value.tags.filter((t) => {
    if (!showSpoiler.value && (t.spoiler_level ?? 0) > 0) return false
    const cat = (t.category || 'content') as TagCategory
    if (!visibleCategories.value.has(cat)) return false
    return true
  })
})

const hiddenByFilterCount = computed(() => {
  if (!detail.value?.tags) return 0
  return detail.value.tags.length - filteredTags.value.length
})

// Wiki frontend origin (used to link to the Wiki galgame detail page)
const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://galgame.kungal.com'
</script>

<template>
  <div v-if="detail" class="space-y-8">
    <section>
      <div class="mb-4 flex flex-wrap items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-2xl font-bold">简介</h2>
        <KunSelect
          :model-value="lang"
          :options="langOptions"
          class-name="max-w-36"
          @update:model-value="(v) => (lang = v as Language)"
        />
      </div>
      <div v-if="introHtml" class="kun-prose max-w-none" v-html="introHtml" />
      <KunNull
        v-else
        description="此 Galgame 暂无简介，可到 Galgame Wiki 补充"
      />

      <div class="text-default-500 mt-6 grid gap-4 sm:grid-cols-2">
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
            Galgame Wiki:
            <a
              :href="`${wikiOrigin}/galgame/${detail.galgame.id}`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              #{{ detail.galgame.id }}（完整角色 / 制作 / 发行信息）
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

    <!-- Tags & officials (developers/publishers) come pre-resolved from Wiki
         by the backend enricher — see apps/api/internal/galgame/enricher/
         enricher.go. Links navigate to moyu's internal /tag/:id and
         /official/:id detail pages. -->
    <section v-if="detail.tags?.length">
      <div
        class="mb-4 flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="flex items-center gap-3">
          <div class="bg-primary h-6 w-1 rounded" />
          <h2 class="text-2xl font-bold">标签</h2>
        </div>
        <div class="flex flex-wrap items-center gap-3 text-sm">
          <label class="flex items-center gap-1">
            <input
              v-model="showSpoiler"
              type="checkbox"
              class="accent-primary"
            />
            <span>显示剧透</span>
          </label>
          <span class="text-default-300">|</span>
          <label
            v-for="c in (['content', 'sexual', 'technical'] as TagCategory[])"
            :key="c"
            class="flex items-center gap-1"
          >
            <input
              type="checkbox"
              :checked="visibleCategories.has(c)"
              class="accent-primary"
              @change="toggleCategory(c)"
            />
            <span :class="TAG_CATEGORY_TEXT_CLASS[c]">
              {{ CATEGORY_LABEL[c] }}
            </span>
          </label>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <NuxtLink
          v-for="t in filteredTags"
          :key="t.id"
          :to="`/tag/${t.id}`"
        >
          <KunChip
            :color="tagColor(t.category)"
            variant="flat"
            size="sm"
          >
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

    <section v-if="detail.officials?.length">
      <div class="mb-4 flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-2xl font-bold">会社</h2>
      </div>
      <div class="flex flex-wrap gap-2">
        <NuxtLink
          v-for="o in detail.officials"
          :key="o.id"
          :to="`/official/${o.id}`"
        >
          <KunChip color="success" variant="flat" size="sm">
            {{ o.name }}
            <span v-if="o.category" class="text-default-500 text-xs">
              · {{ o.category }}
            </span>
          </KunChip>
        </NuxtLink>
      </div>
    </section>

    <!-- W2 / PR3b — screenshots gallery (inline from Wiki PUT-managed list). -->
    <section v-if="detail.galgame?.screenshots?.length">
      <div class="mb-4 flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-2xl font-bold">截图 / 画廊</h2>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <figure
          v-for="s in [...detail.galgame.screenshots].sort(
            (a, b) => a.sort_order - b.sort_order
          )"
          :key="s.image_hash"
          class="border-default/20 overflow-hidden rounded-lg border"
        >
          <img
            :src="imageServiceUrl(s.image_hash)"
            :alt="s.caption || s.image_hash.slice(0, 8)"
            loading="lazy"
            class="bg-default-100 aspect-video w-full object-cover"
          />
          <figcaption
            v-if="s.caption"
            class="text-default-500 px-2 py-1 text-xs"
          >
            {{ s.caption }}
          </figcaption>
        </figure>
      </div>
    </section>

    <!-- Wiki link footer for richer info (characters, staff, releases). -->
    <section v-if="detail.galgame">
      <p class="text-default-500 text-sm">
        角色、制作人员、发行版本等更多信息请查看
        <a
          :href="`${wikiOrigin}/galgame/${detail.galgame.id}`"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary hover:underline"
        >
          Galgame Wiki
        </a>
        。
      </p>
    </section>
  </div>

  <KunNull v-else description="加载失败" />
</template>
