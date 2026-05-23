<script setup lang="ts">
// Patch header actions: favorite / share / edit / delete.
//
// Design: a single icon-only action bar wrapped in tooltips. Previously
// some actions carried a text label (收藏 / 编辑) and others didn't (分享 /
// 删除), and delete used a default-color border despite being destructive.
// This version standardises:
//   - all four buttons are icon-only, same `light` variant, same size
//   - tooltip carries the accessible label, so no per-button text noise
//   - favorite turns danger-red + filled heart only when active (color
//     signal for state)
//   - delete is always danger-red so the destructive intent is obvious
//   - a thin divider separates the read-only actions (favorite / share)
//     from the owner-side actions (edit / delete) without spending a
//     whole new row on grouping
//
// Endpoint contracts unchanged:
//   - favorite: PUT /patch/:id/favorite — local-only state, optimistic UI
//   - share:    copy direct URL to clipboard
//   - edit:     navigates to /edit/rewrite?id=:id (in-site form, proxies to
//               PUT /api/v1/galgame/:gid → Wiki Service per
//               integration-guide.md §6).
//   - delete:   intentionally unimplemented — DELETE /patch/:id exists but
//               the surrounding cleanup (resources / comments / contributor
//               history) needs more design before this is wired up.

interface Props {
  patch: PatchHeader
}

const props = defineProps<Props>()

const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://galgame.kungal.com'

const userStore = useUserStore()
const api = useApi()

const favorite = ref(props.patch.is_favorite)
const favoriteLoading = ref(false)

// Keep local state in sync if the parent re-fetches (e.g. after a PR merge).
watch(
  () => props.patch.is_favorite,
  (v) => {
    favorite.value = v
  }
)

const toggleFavorite = async () => {
  if (!userStore.user.id) {
    useKunMessage('请先登录后再收藏', 'warn')
    return
  }
  favoriteLoading.value = true
  try {
    const res = await api.put<{ favorited: boolean }>(
      `/patch/${props.patch.id}/favorite`
    )
    if (res.code === 0) {
      favorite.value = res.data.favorited
      useKunMessage(favorite.value ? '已收藏' : '已取消收藏', 'success')
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    favoriteLoading.value = false
  }
}

const handleShare = () => {
  const name = getPreferredLanguageText(props.patch.name)
  const link = `${name} - ${window.location.origin}/patch/${props.patch.id}/introduction`
  useKunCopy(link)
}

const editHref = computed(() => `/edit/rewrite?id=${props.patch.id}`)

const handleDelete = () => {
  useKunMessage('暂未实现删除功能', 'warn')
}
</script>

<template>
  <div
    class="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between"
  >
    <!-- Action bar: read-only actions (favorite / share) on the left, an
         in-row divider, then owner-side actions (edit / delete) on the
         right. All icon-only + tooltip — uniform shape avoids the old
         "some labels / some not" inconsistency. -->
    <div
      class="border-default/20 bg-default-50/50 flex items-center gap-1 rounded-xl border p-1"
    >
      <KunTooltip :text="favorite ? '取消收藏' : '收藏 (有新补丁时通知您)'">
        <KunButton
          variant="light"
          :color="favorite ? 'danger' : 'default'"
          size="sm"
          is-icon-only
          :loading="favoriteLoading"
          :disabled="favoriteLoading"
          aria-label="收藏"
          @click="toggleFavorite"
        >
          <KunIcon
            name="lucide:heart"
            :class="cn('size-4', favorite && 'fill-danger text-danger')"
          />
        </KunButton>
      </KunTooltip>

      <KunTooltip text="复制分享链接">
        <KunButton
          variant="light"
          color="default"
          size="sm"
          is-icon-only
          aria-label="复制分享链接"
          @click="handleShare"
        >
          <KunIcon name="lucide:share-2" class="size-4" />
        </KunButton>
      </KunTooltip>

      <div class="bg-default/30 mx-1 h-5 w-px" aria-hidden="true" />

      <KunTooltip text="编辑游戏信息">
        <NuxtLink :to="editHref" aria-label="编辑游戏信息">
          <KunButton
            variant="light"
            color="default"
            size="sm"
            is-icon-only
          >
            <KunIcon name="lucide:pencil" class="size-4" />
          </KunButton>
        </NuxtLink>
      </KunTooltip>

      <KunTooltip text="删除游戏 (暂未实现)">
        <KunButton
          variant="light"
          color="danger"
          size="sm"
          is-icon-only
          aria-label="删除游戏"
          @click="handleDelete"
        >
          <KunIcon name="lucide:trash-2" class="size-4" />
        </KunButton>
      </KunTooltip>
    </div>

    <p class="text-default-500 text-xs">
      游戏元数据由
      <a
        :href="wikiOrigin"
        target="_blank"
        rel="noopener noreferrer"
        class="text-primary hover:underline"
      >
        Galgame Wiki
      </a>
      统一维护
    </p>
  </div>
</template>
