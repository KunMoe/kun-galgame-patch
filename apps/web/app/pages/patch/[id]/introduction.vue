<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

const route = useRoute()
const api = useApi()

const patchId = computed(() => Number(route.params.id))

const { data: detail } = await useAsyncData<PatchDetail | null>(
  () => `patch-detail-${patchId.value}`,
  async () => {
    const res = await api.get<PatchDetail>(`/patch/${patchId.value}/detail`)
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
// data-uid attribute (the renderer adds class="kun-mention").
const introHtml = computed(() => {
  if (!detail.value?.introduction_html) return ''
  const text = getPreferredLanguageText(
    detail.value.introduction_html,
    lang.value
  )
  return DOMPurify.sanitize(text, { ADD_ATTR: ['data-uid'] })
})

const langOptions = [
  { value: 'zh-cn', label: '中文' },
  { value: 'ja-jp', label: '日本語' },
  { value: 'en-us', label: 'English' }
]

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

    <!-- Tags & officials (developers/publishers) come pre-resolved from Wiki by the
         backend enricher — see apps/api/internal/galgame/enricher/enricher.go. -->
    <section v-if="detail.tags?.length">
      <div class="mb-4 flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-2xl font-bold">标签</h2>
      </div>
      <div class="flex flex-wrap gap-2">
        <a
          v-for="t in detail.tags"
          :key="t.id"
          :href="`${wikiOrigin}/tag/${t.id}`"
          target="_blank"
          rel="noopener noreferrer"
        >
          <KunBadge
            :color="t.spoiler_level > 0 ? 'warning' : 'primary'"
            variant="flat"
            size="sm"
          >
            <KunIcon
              v-if="t.spoiler_level > 0"
              name="lucide:eye-off"
              class="size-3.5"
            />
            {{ t.name }}
          </KunBadge>
        </a>
      </div>
    </section>

    <section v-if="detail.officials?.length">
      <div class="mb-4 flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-2xl font-bold">会社</h2>
      </div>
      <div class="flex flex-wrap gap-2">
        <a
          v-for="o in detail.officials"
          :key="o.id"
          :href="`${wikiOrigin}/official/${o.id}`"
          target="_blank"
          rel="noopener noreferrer"
        >
          <KunBadge color="success" variant="flat" size="sm">
            {{ o.name }}
            <span v-if="o.category" class="text-default-500 text-xs">
              · {{ o.category }}
            </span>
          </KunBadge>
        </a>
      </div>
    </section>

    <!-- Wiki link footer for richer info (characters, staff, screenshots, releases). -->
    <section v-if="detail.galgame">
      <p class="text-default-500 text-sm">
        角色、制作人员、截图、发行版本等更多信息请查看
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
