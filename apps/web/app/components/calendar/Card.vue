<script setup lang="ts">
import { GALGAME_AGE_LIMIT_MAP } from '~/constants/galgame'

interface Props {
  item: CalendarItem
  // JST "today" (YYYY-MM-DD) from the calendar response — drives the countdown.
  today?: string
}

const props = defineProps<Props>()

const settingStore = useSettingStore()
const titleLanguage = computed(() => settingStore.data.titleLanguage ?? 'ja-jp')
const name = computed(() =>
  getPreferredLanguageText(props.item.name, titleLanguage.value)
)
const bannerSrc = computed(
  () => resolveBannerUrl(props.item, 'mini') || '/kungalgame-trans.webp'
)

// status=2 = unclaimed VNDB draft. It 404s at /patch/:id, so a draft card routes
// to the publish wizard pre-searched by name → 认领并发布; it shows a 未发布 badge
// and no 收藏 (can't favorite until it exists locally). Published cards link to
// their patch page as usual.
const isDraft = computed(() => props.item.status === 2)
const cardHref = computed(() =>
  isDraft.value
    ? `/edit/create?q=${encodeURIComponent(name.value)}`
    : `/patch/${props.item.id}/introduction`
)

// Countdown relative to `today`, honoring release precision. day → 还有 N 天 /
// 今日发售 / 已发售 N 天; month/year → the fuzzy bucket label; tba/unknown → none.
const ymd = (s: string) => {
  const [y, m, d] = s.split('-').map(Number)
  return y && m && d ? Date.UTC(y, m - 1, d) : NaN
}
const countdown = computed<{
  label: string
  tone: 'today' | 'upcoming' | 'released'
} | null>(() => {
  const g = props.item.galgame
  const precision = g?.release_precision
  const rd = g?.release_date ?? ''
  if (!rd || precision === 'tba' || precision === 'unknown') return null
  if (precision === 'month') return { label: '本月内', tone: 'upcoming' }
  if (precision === 'year') return { label: '年内', tone: 'upcoming' }
  if (!props.today) return null
  const days = Math.round((ymd(rd.slice(0, 10)) - ymd(props.today)) / 86400000)
  if (Number.isNaN(days)) return null
  if (days > 0) return { label: `还有 ${days} 天`, tone: 'upcoming' }
  if (days === 0) return { label: '今日发售', tone: 'today' }
  return { label: '已发售', tone: 'released' }
})
const countdownClass = computed(() => {
  switch (countdown.value?.tone) {
    case 'today':
      return 'bg-primary text-white'
    case 'released':
      return 'bg-background/80 text-default-400'
    default:
      return 'bg-background/90 text-default-700'
  }
})

// Inline 收藏 — favoriting a 未收录 game lazily records it on the backend
// (ensureLocalPatch) AND subscribes the user to its new-patch notifications, so
// the calendar doubles as a one-tap "watch for patch" wishlist. Optimistic;
// reverts on failure; @click.stop so it doesn't trigger the card's NuxtLink.
const api = useApi()
const { requireLogin } = useAuthModal()
const favorited = ref(props.item.is_favorite)
const favPending = ref(false)
watch(
  () => props.item.is_favorite,
  (v) => (favorited.value = v)
)
const toggleFavorite = async () => {
  if (!requireLogin()) return
  if (favPending.value) return
  favPending.value = true
  const next = !favorited.value
  favorited.value = next
  const res = await api.put<{ favorited: boolean }>(
    `/patch/${props.item.id}/favorite`
  )
  favPending.value = false
  if (res.code === 0) {
    favorited.value = res.data.favorited
    useKunMessage(
      favorited.value ? '已收藏，有补丁时第一时间通知你' : '已取消收藏',
      'success'
    )
  } else {
    favorited.value = !next
    useKunMessage(res.message || '操作失败', 'error')
  }
}
</script>

<template>
  <NuxtLink
    :to="cardHref"
    class="group border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block overflow-hidden rounded-lg border transition-colors"
  >
    <div class="relative">
      <KunImage
        :src="bannerSrc"
        :alt="name"
        aspect-ratio="16 / 9"
        :thumbhash="resolveBannerThumbhash(item)"
        class-name="w-full"
      />

      <div class="absolute top-1.5 left-1.5">
        <KunChip
          :color="item.content_limit === 'sfw' ? 'success' : 'danger'"
          variant="flat"
          size="sm"
        >
          {{ GALGAME_AGE_LIMIT_MAP[item.content_limit] }}
        </KunChip>
      </div>

      <!-- Draft (status=2): a 未发布 badge; the card routes to the claim wizard. -->
      <KunChip
        v-if="isDraft"
        color="warning"
        variant="flat"
        size="sm"
        class="absolute top-1.5 right-1.5"
      >
        未发布
      </KunChip>

      <!-- Published: inline 收藏 (watch for patch). Own button, not the card link. -->
      <button
        v-else
        type="button"
        class="bg-background/85 hover:bg-background absolute top-1.5 right-1.5 flex size-7 items-center justify-center rounded-full shadow-kun-sm backdrop-blur transition-colors"
        :aria-label="favorited ? '取消收藏' : '收藏（有补丁时通知你）'"
        @click.stop.prevent="toggleFavorite"
      >
        <KunIcon
          name="lucide:star"
          class="size-4 transition-colors"
          :class="favorited ? 'fill-current text-warning' : 'text-default-500'"
        />
      </button>

      <div
        v-if="countdown"
        class="absolute bottom-1.5 left-1.5 rounded-full px-2 py-0.5 text-xs font-medium shadow-kun-sm backdrop-blur"
        :class="countdownClass"
      >
        {{ countdown.label }}
      </div>
    </div>

    <div class="space-y-1 p-2.5">
      <h3
        class="group-hover:text-primary-500 text-sm font-medium transition-colors line-clamp-2"
      >
        {{ name }}
      </h3>
      <p
        v-if="item.has_patch"
        class="text-primary flex items-center gap-1 text-xs"
      >
        <KunIcon name="lucide:circle-check" class="size-3 shrink-0" />
        本站有补丁
      </p>
    </div>
  </NuxtLink>
</template>
