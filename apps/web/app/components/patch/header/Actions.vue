<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

interface Props {
  patch: PatchHeader
}

const props = defineProps<Props>()

const isOnSite = computed(() => props.patch.is_on_forum !== false)

const kungalOrigin = kunMoyuMoe.domain.kungal

const userStore = useUserStore()
const api = useApi()
const { requireLogin } = useAuthModal()

// Asked after mount, never inside GET /patch/:id: that read runs on every
// navigation, SSR included, and whatever it asks with the reader's token
// spends the catalog allowance they share with the forum. favoriteSeq drops an
// answer that left before a press or a picker save.
const favorite = ref(false)
let favoriteSeq = 0

const loadFavorite = async () => {
  const seq = ++favoriteSeq
  favorite.value = false
  if (!userStore.user.id) return
  const res = await api.get<{ favorited: boolean }>(
    `/patch/${props.patch.id}/favorite`
  )
  if (res.code === 0 && seq === favoriteSeq) {
    favorite.value = res.data.favorited
  }
}

onMounted(loadFavorite)
watch([() => props.patch.id, () => userStore.user.id], loadFavorite)

const onFavoriteChange = async (active: boolean) => {
  favoriteSeq++
  if (!requireLogin()) {
    favorite.value = !active
    return
  }
  const res = await api.put<{ favorited: boolean }>(
    `/patch/${props.patch.id}/favorite`
  )
  if (res.code === 0) {
    favorite.value = res.data.favorited
    useKunMessage(favorite.value ? '已收藏' : '已取消收藏', 'success')
  } else {
    favorite.value = !active
    useKunMessage(res.message || '操作失败', 'error')
  }
}

// The star is "in my default folder"; the picker is the rest of the shelf.
// Both write to the same catalog folders, so the star has to follow whatever
// the picker did.
const pickerOpen = ref(false)
const openPicker = () => {
  if (!requireLogin()) return
  pickerOpen.value = true
}
const onFoldersSaved = (payload: { favorited: boolean }) => {
  favoriteSeq++
  favorite.value = payload.favorited
}

const handleShare = () => {
  const name = getPreferredLanguageText(props.patch.name)
  const link = `${name} - ${window.location.origin}/galgame/${props.patch.id}`
  useKunCopy(link)
}

const editPath = computed(() => `/galgame/${props.patch.id}/edit`)

const canDelete = computed(() => {
  if (!isOnSite.value) return false
  if (!userStore.user.id) return false
  return userStore.isAdmin || props.patch.user?.id === userStore.user.id
})

const deleteOpen = ref(false)
const deleting = ref(false)

const askDelete = () => {
  deleteOpen.value = true
}

const confirmDelete = async () => {
  deleting.value = true
  try {
    const res = await api.delete(`/patch/${props.patch.id}`)
    if (res.code === 0) {
      useKunMessage('已删除游戏', 'success')
      deleteOpen.value = false
      await navigateTo('/')
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    deleting.value = false
  }
}

type ActionMenuItem = {
  key: 'edit' | 'share' | 'delete'
  label: string
  icon: string
  color?: 'default' | 'danger'
}
const menuItems = computed<ActionMenuItem[]>(() => {
  const items: ActionMenuItem[] = [
    { key: 'edit', label: '编辑游戏信息', icon: 'lucide:pencil' },
    { key: 'share', label: '复制分享链接', icon: 'lucide:share-2' }
  ]
  if (canDelete.value) {
    items.push({
      key: 'delete',
      label: '删除游戏',
      icon: 'lucide:trash-2',
      color: 'danger'
    })
  }
  return items
})

const onMenuSelect = (item: { key: string }) => {
  if (item.key === 'edit') navigateTo(editPath.value)
  else if (item.key === 'share') handleShare()
  else if (item.key === 'delete') askDelete()
}
</script>

<template>
  <div
    class="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between"
  >
    <div class="text-default-500 flex flex-col gap-1 text-xs">
      <p v-if="isOnSite">
        资源更新于 {{ formatDistanceToNow(props.patch.resource_update_time) }}
      </p>
      <p>
        游戏元数据由
        <a
          :href="`${kungalOrigin}/galgame`"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary hover:underline"
        >
          鲲 Galgame
        </a>
        统一维护
      </p>
    </div>

    <div class="flex flex-col items-start gap-2 sm:items-end">
      <div class="flex items-center gap-2">
        <PatchDlsitePurchase
          v-if="props.patch.dlsite_purchase_url"
          :purchase-url="props.patch.dlsite_purchase_url"
          :coupon-url="props.patch.dlsite_coupon_url"
          :campaign-name="props.patch.dlsite_campaign_name"
        />

        <KunReaction
          v-model="favorite"
          icon="lucide:star"
          color="warning"
          size="md"
          @change="onFavoriteChange"
        >
          {{ favorite ? '已收藏' : '收藏游戏' }}
        </KunReaction>

        <KunButton
          variant="light"
          size="md"
          aria-label="加入收藏夹"
          @click="openPicker"
        >
          <KunIcon name="lucide:folder-plus" class="size-4" />
          收藏夹
        </KunButton>

        <KunDropdown
          :items="menuItems"
          position="bottom-end"
          @select="onMenuSelect"
        >
          <template #trigger>
            <KunButton
              is-icon-only
              variant="light"
              color="default"
              size="sm"
              rounded="full"
              aria-label="更多操作"
            >
              <KunIcon name="lucide:ellipsis-vertical" class="size-4" />
            </KunButton>
          </template>
        </KunDropdown>
      </div>

      <p
        :class="
          cn(
            'flex items-center gap-1.5 text-xs',
            favorite ? 'text-primary' : 'text-default-500'
          )
        "
      >
        <KunIcon
          :name="favorite ? 'lucide:bell-ring' : 'lucide:bell'"
          class="size-3.5 shrink-0"
        />
        <span>{{
          favorite
            ? '已收藏，有新补丁时会通知你'
            : '收藏后，有新补丁时第一时间通知你'
        }}</span>
      </p>
    </div>
  </div>

  <KunModal
    v-model="deleteOpen"
    inner-class-name="max-w-md"
    :is-dismissable="false"
    aria-label="删除该游戏"
  >
    <div class="space-y-4 py-2">
      <h3 class="text-lg font-bold">删除该游戏？</h3>
      <p class="text-default-600 text-sm">
        此操作不可撤销。本站会删除该游戏的所有补丁资源、评论、贡献者记录、收藏关系。
      </p>
      <p class="text-default-500 text-xs">
        这只会删除本站记录，不会影响 Galgame 资料库上的游戏条目。
      </p>
      <div class="flex justify-end gap-2">
        <KunButton
          variant="light"
          color="default"
          :disabled="deleting"
          @click="deleteOpen = false"
        >
          取消
        </KunButton>
        <KunButton
          color="danger"
          :loading="deleting"
          :disabled="deleting"
          @click="confirmDelete"
        >
          删除
        </KunButton>
      </div>
    </div>
  </KunModal>

    <PatchFolderPickerModal
      v-model="pickerOpen"
      :patch-id="props.patch.id"
      @saved="onFoldersSaved"
    />
</template>
