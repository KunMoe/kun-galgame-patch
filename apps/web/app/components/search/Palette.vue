<script setup lang="ts">
import type { KunCommandGroup, KunCommandItem } from '@kungal/ui-vue'
import { watchDebounced } from '@vueuse/core'

// Mirrors the API's `max=107` on keywords: past it the request is a validation
// error, and one per keystroke would be an error toast per keystroke.
const MAX_KEYWORDS_LENGTH = 107

const api = useApi()
const searchStore = useSearchStore()

const open = ref(false)
const query = ref('')
const pending = ref(false)
const result = ref<SearchLanes | null>(null)

const keywords = computed(() => query.value.trim())

let latest = 0

const search = async (value: string) => {
  const current = ++latest
  if (!value || value.length > MAX_KEYWORDS_LENGTH) {
    pending.value = false
    result.value = null
    return
  }

  pending.value = true
  const res = await api.get<SearchLanes>(
    `/search/quick?keywords=${encodeURIComponent(value)}`
  )
  if (current !== latest) {
    return
  }
  pending.value = false
  result.value = res.code === 0 ? res.data : null
}

watchDebounced(keywords, search, { debounce: 300 })

watch(open, (isOpen) => {
  if (!isOpen) {
    latest++
    pending.value = false
    result.value = null
  }
})

const hasHit = computed(() => {
  const hit = result.value
  return (
    !!hit && hit.galgames.length + hit.resources.length + hit.users.length > 0
  )
})

// The action row is always present, so the palette's own no-result text can
// never show. Say it here instead.
const actionDescription = computed(() =>
  !pending.value && result.value && !hasHit.value
    ? '快速搜索没有匹配, 换个关键词试试'
    : '在搜索页面查看 Galgame, 补丁资源与用户的全部结果'
)

const label = (name: string, total: number) =>
  total > 0 ? `${name} · ${total}` : name

const groups = computed<KunCommandGroup[]>(() => {
  if (!keywords.value) {
    if (!searchStore.history.length) {
      return []
    }
    return [
      {
        label: '搜索历史',
        items: searchStore.history
          .slice()
          .reverse()
          .map((history) => ({
            value: `q:${history}`,
            label: history,
            icon: 'lucide:history'
          }))
      }
    ]
  }

  const list: KunCommandGroup[] = [
    {
      items: [
        {
          value: `q:${keywords.value}`,
          label: `搜索「${keywords.value}」`,
          description: actionDescription.value,
          icon: 'lucide:search'
        }
      ]
    }
  ]
  if (!result.value) {
    return list
  }

  const { galgames, resources, users, totals } = result.value
  if (galgames.length) {
    list.push({
      label: label('Galgame', totals.galgame),
      items: galgames.map((galgame) => {
        const title = getPreferredLanguageText(galgame.name)
        const maker = resolveMaker(galgame)
        return {
          value: `/galgame/${galgame.id}`,
          label: title,
          // The credited company, or the original title when the reader's
          // 标题语言 is showing them a translation of it.
          description: maker
            ? getPreferredLanguageText(maker.name)
            : galgame.name['ja-jp'] === title
              ? ''
              : galgame.name['ja-jp'],
          icon: 'lucide:gamepad-2'
        }
      })
    })
  }
  if (resources.length) {
    list.push({
      label: label('补丁资源', totals.resource),
      items: resources.map((resource) => ({
        value: `/resource/${resource.id}`,
        label:
          resource.name ||
          (resource.patch
            ? getPreferredLanguageText(resource.patch.name)
            : '') ||
          '补丁资源',
        description: resource.patch
          ? getPreferredLanguageText(resource.patch.name)
          : '',
        icon: 'lucide:puzzle'
      }))
    })
  }
  if (users.length) {
    list.push({
      label: label('用户', totals.user),
      items: users.map((user) => ({
        value: `/user/${user.id}/resource`,
        label: user.name,
        description: user.bio,
        icon: 'lucide:user-round'
      }))
    })
  }
  return list
})

// The palette never navigates while typing. The first row is always the search
// action, so a bare Enter goes to the search page and only an explicitly moved
// selection opens a single result.
const handleSelect = (item: KunCommandItem) => {
  const value = String(item.value)
  if (!value.startsWith('q:')) {
    navigateTo(value)
    return
  }

  const typed = value.slice(2)
  searchStore.remember(typed)
  navigateTo({ path: '/search', query: { q: typed } })
}
</script>

<template>
  <KunCommandPalette
    v-model:open="open"
    v-model:query="query"
    :items="groups"
    :loading="pending && !hasHit"
    placeholder="搜索 Galgame, 补丁资源, 用户…"
    empty-text="输入关键字以搜索整站"
    aria-label="站内搜索"
    @select="handleSelect"
  >
    <template #trigger="{ open: openPalette, shortcut }">
      <KunTooltip :text="`${shortcut} 快速搜索`" position="bottom">
        <KunButton
          :is-icon-only="true"
          variant="light"
          color="default"
          size="sm"
          aria-label="搜索"
          @click="openPalette"
        >
          <KunIcon name="lucide:search" class="text-default-500 size-6" />
        </KunButton>
      </KunTooltip>
    </template>
  </KunCommandPalette>
</template>
