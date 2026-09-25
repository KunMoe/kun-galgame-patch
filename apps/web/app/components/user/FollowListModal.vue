<script setup lang="ts">
interface FollowItem {
  id: number
  name: string
  avatar: string
  cosmetics?: UserCosmetics
  is_followed: boolean
}

interface FollowListResponse {
  items: FollowItem[]
  next_cursor?: string
  total: number
}

interface Props {
  userId: number
  mode: 'follower' | 'following'
}

const props = defineProps<Props>()
const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{
  followChanged: []
}>()

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const limit = 20
const items = ref<FollowItem[]>([])
const total = ref(0)
const cursor = ref('')
const pending = ref(false)
const loadingMore = ref(false)
const hasMore = computed(() => cursor.value !== '')

const load = async (append = false) => {
  if (append) {
    loadingMore.value = true
  } else {
    pending.value = true
  }
  try {
    const query = new URLSearchParams({ limit: String(limit) })
    if (append && cursor.value) {
      query.set('cursor', cursor.value)
    }
    const res = await api.get<FollowListResponse>(
      `/user/${props.userId}/${props.mode}?${query.toString()}`
    )
    if (res.code === 0 && res.data) {
      const page = res.data.items ?? []
      items.value = append ? [...items.value, ...page] : page
      total.value = res.data.total ?? 0
      cursor.value = res.data.next_cursor ?? ''
    } else if (!append) {
      items.value = []
      total.value = 0
      cursor.value = ''
    }
  } finally {
    pending.value = false
    loadingMore.value = false
  }
}

watch(
  () => [open.value, props.mode, props.userId] as const,
  ([isOpen]) => {
    if (isOpen) {
      items.value = []
      total.value = 0
      cursor.value = ''
      load()
    }
  },
  { immediate: true }
)

const title = computed(() => (props.mode === 'follower' ? '粉丝' : '关注'))

const toggling = ref<number | null>(null)
const toggleFollow = async (row: FollowItem) => {
  if (!requireLogin()) return
  if (row.id === userStore.user.id) return
  toggling.value = row.id
  const wasFollowed = row.is_followed
  row.is_followed = !wasFollowed
  try {
    const res = wasFollowed
      ? await api.delete(`/user/${row.id}/follow`)
      : await api.put(`/user/${row.id}/follow`)
    if (res.code !== 0) {
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
  <KunModal v-model="open" inner-class-name="max-w-md" :aria-label="title">
    <div class="space-y-4">
      <h3 class="text-lg font-semibold">
        {{ title }}
        <span class="text-default-500 text-sm font-normal">{{ total }}</span>
      </h3>

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
              :user="toKunUser(row)"
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

      <div v-if="hasMore" class="flex justify-center">
        <KunButton
          variant="light"
          color="primary"
          :loading="loadingMore"
          :disabled="loadingMore"
          @click="load(true)"
        >
          加载更多
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
