<script setup lang="ts">
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

defineOptions({ name: 'user-folder' })

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const userId = computed(() => Number(route.params.id))
const isOwner = computed(() => userStore.user.id === userId.value)

const { data, pending, refresh } = await useAsyncData<{ folders: Folder[] }>(
  () => `user-${userId.value}-folders`,
  async () => {
    const res = await api.get<{ folders: Folder[] }>(
      `/user/${userId.value}/folder`
    )
    return res.code === 0 ? res.data : { folders: [] }
  },
  { default: () => ({ folders: [] }) }
)

// The default folder carries an empty name on purpose — every client derives
// the label, so a name invented at import time would freeze one language into
// everybody's view of somebody else's shelf.
const labelOf = (folder: Folder) =>
  folder.name.trim() || (folder.is_default ? '默认收藏夹' : '未命名收藏夹')

const coversOf = (folder: Folder) =>
  (folder.preview_covers ?? [])
    .slice(0, 4)
    .map((hash) => imageServiceUrl(hash, 'mini'))
    .filter(Boolean)

const visibilityOf = (folder: Folder) =>
  folder.visibility === 'private'
    ? { icon: 'lucide:lock', label: '私密' }
    : { icon: 'lucide:globe', label: '公开' }

const creating = ref(false)
const newName = ref('')

const createFolder = async () => {
  const name = newName.value.trim()
  if (!name) return
  creating.value = true
  const res = await api.post<Folder>('/folder', {
    name,
    description: '',
    visibility: 'public'
  })
  creating.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '创建收藏夹失败', 'error')
    return
  }
  newName.value = ''
  await refresh()
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="isOwner" class="flex items-center gap-2">
      <KunInput
        v-model="newName"
        placeholder="新建收藏夹..."
        class="max-w-xs flex-1"
        @keyup.enter="createFolder"
      />
      <KunButton
        color="primary"
        :loading="creating"
        :disabled="!newName.trim()"
        @click="createFolder"
      >
        新建
      </KunButton>
    </div>

    <KunLoading v-if="pending" description="加载中..." />

    <div
      v-else-if="data?.folders?.length"
      class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
    >
      <KunCard
        v-for="folder in data.folders"
        :key="folder.id"
        :href="`/folder/${folder.id}`"
        is-hoverable
        content-class="space-y-3"
      >
        <div
          class="bg-default-100 grid aspect-video grid-cols-2 grid-rows-2 gap-0.5 overflow-hidden rounded-lg"
        >
          <img
            v-for="(cover, index) in coversOf(folder)"
            :key="index"
            :src="cover"
            alt=""
            loading="lazy"
            :class="
              cn(
                'size-full object-cover',
                coversOf(folder).length === 1 && 'col-span-2 row-span-2'
              )
            "
          />
          <div
            v-if="!coversOf(folder).length"
            class="text-default-300 col-span-2 row-span-2 flex items-center justify-center"
          >
            <KunIcon name="lucide:heart" class="size-10" />
          </div>
        </div>

        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <span class="text-foreground truncate font-medium">
              {{ labelOf(folder) }}
            </span>
            <span v-if="folder.is_default" class="text-default-400 shrink-0 text-xs">
              默认
            </span>
          </div>
          <p
            v-if="folder.description"
            class="text-default-500 line-clamp-2 text-xs"
          >
            {{ folder.description }}
          </p>
          <div class="text-default-500 flex items-center gap-1.5 text-xs">
            <KunIcon :name="visibilityOf(folder).icon" class="size-3.5" />
            <span>{{ visibilityOf(folder).label }}</span>
            <span class="ml-auto">{{ folder.item_count }} 个游戏</span>
          </div>
        </div>
      </KunCard>
    </div>

    <KunNull
      v-else
      :description="isOwner ? '你还没有收藏夹' : '该用户没有公开的收藏夹'"
    />
  </div>
</template>
