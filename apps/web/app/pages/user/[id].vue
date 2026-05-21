<script setup lang="ts">
import { pickRoleLabel } from '~/constants/user'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

const userId = computed(() => Number(route.params.id))

const { data: user, refresh } = await useAsyncData<UserInfo | null>(
  () => `user-${userId.value}`,
  async () => {
    const res = await api.get<UserInfo>(`/user/${userId.value}`)
    return res.code === 0 ? res.data : null
  }
)

useKunSeoMeta({
  title: user.value?.name ?? `用户 ${userId.value}`,
  description: user.value?.bio ?? ''
})

const isSelf = computed(
  () => user.value && user.value.id === userStore.user.uid
)

const tabs = computed(() => [
  { key: 'resource', title: '补丁资源', href: `/user/${userId.value}/resource` },
  { key: 'galgame', title: 'Galgame', href: `/user/${userId.value}/galgame` },
  { key: 'contribute', title: '贡献', href: `/user/${userId.value}/contribute` },
  { key: 'favorite', title: '收藏', href: `/user/${userId.value}/favorite` },
  { key: 'comment', title: '评论', href: `/user/${userId.value}/comment` }
])

const currentTab = computed(() =>
  route.path.split('/').filter(Boolean).pop() ?? 'resource'
)

// 发消息: resolves or creates the private chat room between the current
// user and the profile owner, then navigates to its transcript. Backend
// endpoint is POST /chat/room/private (link format "<minUID>-<maxUID>"
// converges both directions).
const startingChat = ref(false)
const handleStartPrivateChat = async () => {
  if (!userStore.user.uid) {
    useKunMessage('请先登录', 'warn')
    return
  }
  if (!user.value) return
  if (user.value.id === userStore.user.uid) {
    useKunMessage('不能给自己发消息', 'warn')
    return
  }
  startingChat.value = true
  try {
    const res = await api.post<{ link: string }>('/chat/room/private', {
      peer_uid: user.value.id
    })
    if (res.code === 0 && res.data?.link) {
      await navigateTo(`/message/chat/${res.data.link}`)
    } else {
      useKunMessage(res.message || '打开私聊失败', 'error')
    }
  } finally {
    startingChat.value = false
  }
}

const followLoading = ref(false)
const toggleFollow = async () => {
  if (!userStore.user.uid) {
    useKunMessage('请先登录', 'warn')
    return
  }
  if (!user.value) return
  followLoading.value = true
  try {
    // Backend uses PUT to follow and DELETE to unfollow — see router.go.
    const res = user.value.is_followed
      ? await api.delete(`/user/${user.value.id}/follow`)
      : await api.put(`/user/${user.value.id}/follow`)
    if (res.code === 0) {
      await refresh()
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    followLoading.value = false
  }
}
</script>

<template>
  <div v-if="user" class="container mx-auto my-4 space-y-6">
    <div class="grid gap-4 lg:grid-cols-3">
      <div class="lg:col-span-1">
        <KunCard :bordered="true">
          <div class="flex flex-col items-center gap-3 pt-4">
            <KunAvatar :user="user" size="original-sm" :is-navigation="false" />
            <div class="flex flex-col items-center gap-1">
              <h4 class="text-2xl font-bold">{{ user.name }}</h4>
              <KunChip color="primary" variant="flat" size="sm">
                {{ pickRoleLabel(user.roles) }}
              </KunChip>

              <div class="text-default-500 mt-2 flex gap-4 text-sm">
                <span>粉丝 {{ user.follower_count }}</span>
                <span>关注 {{ user.following_count }}</span>
              </div>
            </div>
          </div>
          <p
            v-if="user.bio"
            class="text-default-600 mt-4 text-center"
          >
            {{ user.bio }}
          </p>
          <div class="text-default-500 mt-4 space-y-2 text-sm">
            <div class="flex items-center gap-2">
              <KunIcon name="lucide:calendar" class="size-4" />
              加入于
              {{
                formatDate(user.register_time, {
                  isShowYear: true,
                  isPrecise: true
                })
              }}
            </div>
            <div class="flex items-center gap-2">
              <KunIcon name="lucide:lollipop" class="size-4" />
              萌萌点 {{ user.moemoepoint }}
            </div>
          </div>

          <div class="mt-4 flex gap-2">
            <NuxtLink v-if="isSelf" to="/settings/user" class="flex-1">
              <KunButton variant="flat" color="primary" full-width>
                编辑资料
              </KunButton>
            </NuxtLink>
            <template v-else>
              <KunButton
                :variant="user.is_followed ? 'flat' : 'solid'"
                color="primary"
                full-width
                :loading="followLoading"
                @click="toggleFollow"
              >
                {{ user.is_followed ? '已关注' : '关注' }}
              </KunButton>
              <KunButton
                color="primary"
                variant="bordered"
                full-width
                :loading="startingChat"
                :disabled="startingChat"
                @click="handleStartPrivateChat"
              >
                <KunIcon name="lucide:message-circle" class="size-4" />
                发消息
              </KunButton>
            </template>
          </div>
        </KunCard>

        <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-2">
          <KunCard :bordered="true">
            <div class="flex items-center gap-3 p-1">
              <KunIcon name="lucide:puzzle" class="text-primary size-6" />
              <div>
                <div class="text-xl font-bold">
                  {{ user.resource_count }}
                </div>
                <div class="text-default-500 text-xs">补丁资源</div>
              </div>
            </div>
          </KunCard>
          <KunCard :bordered="true">
            <div class="flex items-center gap-3 p-1">
              <KunIcon name="lucide:gamepad-2" class="text-primary size-6" />
              <div>
                <div class="text-xl font-bold">
                  {{ user.patch_count }}
                </div>
                <div class="text-default-500 text-xs">Galgame</div>
              </div>
            </div>
          </KunCard>
          <KunCard :bordered="true">
            <div class="flex items-center gap-3 p-1">
              <KunIcon name="lucide:message-circle" class="text-primary size-6" />
              <div>
                <div class="text-xl font-bold">
                  {{ user.comment_count }}
                </div>
                <div class="text-default-500 text-xs">评论</div>
              </div>
            </div>
          </KunCard>
          <KunCard :bordered="true">
            <div class="flex items-center gap-3 p-1">
              <KunIcon name="lucide:star" class="text-primary size-6" />
              <div>
                <div class="text-xl font-bold">
                  {{ user.favorite_count }}
                </div>
                <div class="text-default-500 text-xs">收藏</div>
              </div>
            </div>
          </KunCard>
        </div>
      </div>

      <div class="lg:col-span-2">
        <nav class="border-default/20 mb-4 flex gap-3 overflow-x-auto border-b">
          <NuxtLink
            v-for="t in tabs"
            :key="t.key"
            :to="t.href"
            :class="
              cn(
                'whitespace-nowrap px-3 py-3 text-sm transition-colors',
                currentTab === t.key
                  ? 'text-primary border-primary -mb-px border-b-2 font-medium'
                  : 'text-default-600 hover:text-foreground'
              )
            "
          >
            {{ t.title }}
          </NuxtLink>
        </nav>
        <NuxtPage />
      </div>
    </div>
  </div>
  <KunNull v-else description="用户不存在" />
</template>
