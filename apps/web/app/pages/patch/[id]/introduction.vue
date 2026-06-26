<script setup lang="ts">
import { useContentBlurUp } from '@kungal/ui-vue'

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

// ThumbHash blur-up for the intro's body images (KunUI decodes the
// data-thumbhash the API emits on each <img>; width/height reserve the ratio).
const introEl = ref<HTMLElement | null>(null)
useContentBlurUp(introEl)

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

// introduction_html is pre-rendered server-side via the markdown package
// (goldmark, no html.WithUnsafe → raw HTML escaped + dangerous URLs dropped),
// so it's already safe and bound directly. The renderer keeps mention links'
// class="kun-mention" + data-id.
const introHtml = computed(() => {
  if (!detail.value?.introduction_html) return ''
  return getPreferredLanguageText(detail.value.introduction_html, lang.value)
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

// SFW (safe) mode never shows sexual tags — not even behind the toggle. The
// category checkboxes only offer the categories allowed in the current mode.
const isSafeMode = computed(() => settingStore.data.kunNsfwEnable === 'sfw')
const availableCategories = computed<TagCategory[]>(() =>
  isSafeMode.value
    ? ['content', 'technical']
    : ['content', 'sexual', 'technical']
)

// Spoiler filtering follows VNDB's 3-level model (tag.spoiler_level 0/1/2 =
// none / minor / major). The control picks the MAX level to reveal, so it's a
// graduated 剧透等级 filter, not just a show/hide toggle:
//   none  → only spoiler_level 0   (default; safe)
//   minor → spoiler_level <= 1
//   all   → everything (incl. major)
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
    const cat = (t.category || 'content') as TagCategory
    // SFW mode hard-hides sexual tags regardless of the manual toggle.
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

// Wiki frontend origin (used to link to the Wiki galgame detail page)
const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://wiki.kungal.com'
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
      <div
        v-if="introHtml"
        ref="introEl"
        class="kun-prose max-w-none"
        v-html="introHtml"
      />
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
        <div class="flex flex-wrap items-center gap-x-6 gap-y-3 text-sm">
          <!-- 剧透等级：互斥单选（隐藏 / 轻微 / 完全） -->
          <div class="flex items-center gap-2">
            <span class="text-default-500 shrink-0">剧透</span>
            <KunRadioGroup
              v-model="spoilerMode"
              orientation="horizontal"
              :options="spoilerOptions"
            />
          </div>
          <!-- 分类：多选。span 仅用于按类别上色；方框与文字的间距由
               KunCheckBox 自带的 gap 提供（@kungal/ui-vue >= 0.6.2）。 -->
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

    <!-- W2 / PR3b — screenshots gallery (inline from Wiki PUT-managed list).
         Per-rating SFW gate (色情 + 暴力 axes) + the shared lightbox live in
         GalgameGallery (ported from kungal); it renders its own 截图/画廊
         header and the 分级筛选 control. -->
    <section v-if="detail.galgame?.screenshots?.length">
      <GalgameGallery :screenshots="detail.galgame.screenshots" />
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
