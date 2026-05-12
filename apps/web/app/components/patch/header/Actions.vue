<script setup lang="ts">
// Patch header actions: favorite / share / edit / delete.
//
//   - favorite: PUT /patch/:id/favorite — local-only state, optimistic UI
//   - share:    copy direct URL to clipboard
//   - edit:     navigates to the in-site /edit/rewrite?id=:id form. The form
//               proxies to PUT /api/v1/galgame/:gid which forwards to the
//               Galgame Wiki Service (per integration-guide.md §6, edits go
//               through our backend so we can attach local side effects when
//               needed). Tag/official/engine/banner edits aren't covered by
//               the in-site form and link out to the Wiki edit page.
//   - delete:   intentionally unimplemented — clicking shows a toast. The
//               backend route DELETE /patch/:id exists but the surrounding
//               cleanup (resources / comments / contributor history) needs
//               more design before the button is wired up.

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
  if (!userStore.user.uid) {
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
  <div class="flex flex-col items-start gap-3 sm:flex-row sm:items-center">
    <div class="flex flex-wrap items-center gap-2">
      <KunTooltip :text="favorite ? '取消收藏' : '收藏 (有新补丁时通知您)'">
        <KunButton
          :color="favorite ? 'danger' : 'default'"
          :variant="favorite ? 'flat' : 'bordered'"
          size="sm"
          :loading="favoriteLoading"
          :disabled="favoriteLoading"
          aria-label="收藏"
          @click="toggleFavorite"
        >
          <KunIcon
            name="lucide:heart"
            :class="
              cn('size-4', favorite ? 'fill-danger-500 text-danger-500' : '')
            "
          />
          <span>{{ favorite ? '已收藏' : '收藏' }}</span>
        </KunButton>
      </KunTooltip>

      <KunTooltip text="复制分享链接">
        <KunButton
          variant="bordered"
          size="sm"
          aria-label="复制分享链接"
          @click="handleShare"
        >
          <KunIcon name="lucide:share-2" class="size-4" />
        </KunButton>
      </KunTooltip>

      <KunTooltip text="编辑游戏信息">
        <NuxtLink :to="editHref" aria-label="编辑游戏信息">
          <KunButton variant="bordered" size="sm">
            <KunIcon name="lucide:pencil" class="size-4" />
            <span>编辑</span>
          </KunButton>
        </NuxtLink>
      </KunTooltip>

      <KunTooltip text="删除游戏 (暂未实现)">
        <KunButton
          variant="bordered"
          size="sm"
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
