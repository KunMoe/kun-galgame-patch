<script setup lang="ts">
// Modal for viewing a user's follower / following list, with a per-row
// follow / unfollow button. Driven by the user profile page (clicking
// 粉丝 N or 关注 N).
//
// The backend (WhichFollowed + GetFollowers/GetFollowing) stamps each
// row with `is_followed` relative to the current viewer, so the button
// state is correct on first render — no per-row round-trip.

interface FollowItem {
  id: number
  name: string
  avatar: string
  is_followed: boolean
}

interface PaginatedResponse {
  items: FollowItem[]
  total: number
}

interface Props {
  userId: number
  // 'follower' = users who follow `userId`. 'following' = users `userId` follows.
  mode: 'follower' | 'following'
}

const props = defineProps<Props>()
const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{
  // Fired after a follow/unfollow so the parent page can refresh counts.
  followChanged: []
}>()

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const page = ref(1)
const limit = 20
const items = ref<FollowItem[]>([])
const total = ref(0)
const pending = ref(false)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

const endpoint = computed(
  () =>
    `/user/${props.userId}/${props.mode}?page=${page.value}&limit=${limit}`
)

const load = async () => {
  pending.value = true
  try {
    const res = await api.get<PaginatedResponse>(endpoint.value)
    if (res.code === 0 && res.data) {
      items.value = res.data.items ?? []
      total.value = res.data.total ?? 0
    } else {
      items.value = []
      total.value = 0
    }
  } finally {
    pending.value = false
  }
}

// Reset page + load when opened or mode switches; if `open` flips false
// we leave the items in place (no flicker on re-open if backed by cache).
watch(
  () => [open.value, props.mode, props.userId] as const,
  ([isOpen]) => {
    if (isOpen) {
      page.value = 1
      load()
    }
  },
  { immediate: true }
)
watch(page, () => {
  if (open.value) load()
})

const title = computed(() => (props.mode === 'follower' ? '粉丝' : '关注'))

// Per-row follow toggle. Optimistic update — flip the local row + count
// first, revert on failure. Backend's PUT is idempotent (already-followed
// returns success), DELETE likewise (not-following returns success).
const toggling = ref<number | null>(null)
const toggleFollow = async (row: FollowItem) => {
  if (!requireLogin()) return
  if (row.id === userStore.user.id) return // can't follow self
  toggling.value = row.id
  const wasFollowed = row.is_followed
  row.is_followed = !wasFollowed
  try {
    const res = wasFollowed
      ? await api.delete(`/user/${row.id}/follow`)
      : await api.put(`/user/${row.id}/follow`)
    if (res.code !== 0) {
      // Revert
      row.is_followed = wasFollowed
      useKunMessage(res.message || '操作失败', 'error')
      return
    }
    emit('followChanged')
  } finally {
    toggling.value = null
  }
}

const goToProfile = (id: number) => {
  open.value = false
  navigateTo(`/user/${id}/resource`)
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-md">
    <div class="space-y-4">
      <h3 class="text-lg font-semibold">{{ title }}</h3>

      <KunLoading v-if="pending && !items.length" description="加载中..." />

      <KunNull
        v-else-if="!items.length"
        :description="
          mode === 'follower' ? '还没有粉丝' : '还没有关注任何人'
        "
      />

      <div v-else class="max-h-[60vh] space-y-2 overflow-y-auto">
        <div
          v-for="row in items"
          :key="row.id"
          class="hover:bg-default-50 flex items-center gap-3 rounded-lg p-2 transition-colors"
        >
          <button
            type="button"
            class="flex min-w-0 flex-1 items-center gap-3 text-left"
            @click="goToProfile(row.id)"
          >
            <KunAvatar
              :user="{ id: row.id, name: row.name, avatar: row.avatar }"
              :is-navigation="false"
              size="sm"
            />
            <span class="truncate text-sm font-medium">{{ row.name }}</span>
          </button>

          <KunButton
            v-if="userStore.user.id && row.id !== userStore.user.id"
            :variant="row.is_followed ? 'flat' : 'solid'"
            color="primary"
            size="sm"
            :loading="toggling === row.id"
            :disabled="toggling === row.id"
            @click="toggleFollow(row)"
          >
            {{ row.is_followed ? '已关注' : '关注' }}
          </KunButton>
        </div>
      </div>

      <KunPagination
        v-if="totalPage > 1"
        v-model:current-page="page"
        :total-page="totalPage"
        :is-loading="pending"
      />
    </div>
  </KunModal>
</template>
