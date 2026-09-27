<script setup lang="ts">
import { pickRoleBadge } from '~/constants/user'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const userId = computed(() => Number(route.params.id))

const { data: user, refresh } = await useAsyncData<UserInfo | null>(
  () => `user-${userId.value}`,
  async () => {
    const res = await api.get<UserInfo>(`/user/${userId.value}`)
    return res.code === 0 ? res.data : null
  }
)

if (user.value && user.value.name) {
  useKunSeoMeta({
    title: `${user.value.name} 的主页`,
    description:
      user.value.bio ||
      `${user.value.name} 在鲲 Galgame 补丁站发布的补丁资源、Galgame 与评论`,
    ogImage: user.value.avatar || undefined
  })
} else {
  useKunDisableSeo(`用户 ${userId.value}`)
}

const background = computed(() => user.value?.cosmetics?.profile_background)

const isSelf = computed(
  () => user.value && user.value.id === userStore.user.id
)

const tabs = computed(() => [
  { key: 'info', title: '动态', href: `/user/${userId.value}/info` },
  { key: 'resource', title: '补丁资源', href: `/user/${userId.value}/resource` },
  { key: 'galgame', title: 'Galgame', href: `/user/${userId.value}/galgame` },
  { key: 'contribute', title: '贡献', href: `/user/${userId.value}/contribute` },
  { key: 'favorite', title: '收藏', href: `/user/${userId.value}/favorite` },
  { key: 'folder', title: '收藏夹', href: `/user/${userId.value}/folder` },
  { key: 'comment', title: '评论', href: `/user/${userId.value}/comment` }
])

const currentTab = computed({
  get: () => route.path.split('/').filter(Boolean).pop() ?? 'resource',
  set: () => {}
})

const followOpen = ref(false)
const followMode = ref<'follower' | 'following'>('follower')
const openFollowList = (mode: 'follower' | 'following') => {
  followMode.value = mode
  followOpen.value = true
}

const chatEnabled = useRuntimeConfig().public.chatEnabled
const chatActions = useChatActions()
const startingChat = ref(false)
const handleStartPrivateChat = async () => {
  if (!requireLogin()) return
  if (!user.value) return
  if (user.value.id === userStore.user.id) {
    useKunMessage('不能给自己发消息', 'warn')
    return
  }
  startingChat.value = true
  if (chatEnabled) {
    const peer = user.value.id
    const r = await chatActions.openDirect(peer)
    startingChat.value = false
    if (r.code === 0) await navigateTo(`/messages/${r.data.id}`)
    else if (r.code === 40313) await navigateTo(`/messages?peer=${peer}`)
    else useKunMessage(r.message || '无法发起私信', 'warn')
    return
  }
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
  if (!requireLogin()) return
  if (!user.value) return
  followLoading.value = true
  try {
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

const notifyAll = computed(() => user.value?.follow_notify !== 'feed')
const notifyLoading = ref(false)
const toggleNotify = async () => {
  if (!user.value?.is_followed || notifyLoading.value) return
  notifyLoading.value = true
  const notify = notifyAll.value ? 'feed' : 'all'
  try {
    const res = await api.patch<{ notify: 'all' | 'feed' }>(
      `/user/${user.value.id}/follow`,
      { notify }
    )
    if (res.code !== 0) {
      useKunMessage(res.message || '操作失败', 'error')
      await refresh()
      return
    }
    user.value.follow_notify = res.data.notify
    useKunMessage(
      res.data.notify === 'all'
        ? 'TA 发布新内容时会通知你'
        : '不再通知, TA 的动态仍会出现在「关注动态」里',
      'success'
    )
  } finally {
    notifyLoading.value = false
  }
}
</script>

<template>
  <div v-if="user" class="container mx-auto my-4 space-y-6">
    <div class="grid gap-4 lg:grid-cols-3">
      <div class="lg:col-span-1">
        <KunCard :bordered="true">
          <picture
            v-if="background"
            class="block aspect-[3/1] overflow-hidden rounded-lg"
          >
            <source
              v-if="background.animated_url"
              media="(prefers-reduced-motion: reduce)"
              :srcset="background.static_url"
            />
            <img
              :src="background.animated_url ?? background.static_url"
              alt=""
              decoding="async"
              class="size-full object-cover object-center"
            />
          </picture>
          <div class="flex items-center gap-4 pt-4">
            <KunAvatar
              :user="toKunUser(user)"
              size="original-sm"
              :is-navigation="false"
              class-name="shrink-0"
            />
            <div class="flex min-w-0 flex-col gap-2">
              <div class="flex flex-wrap items-center gap-2">
                <h4 class="text-2xl font-bold break-words">{{ user.name }}</h4>
                <KunChip
                  :color="
                    pickRoleBadge(user.roles, user.site_roles).site
                      ? 'secondary'
                      : 'primary'
                  "
                  variant="flat"
                  size="sm"
                >
                  {{ pickRoleBadge(user.roles, user.site_roles).label }}
                </KunChip>
              </div>

              <div class="text-default-500 flex gap-3 text-sm">
                <button
                  type="button"
                  class="hover:text-primary inline-flex items-center gap-1 rounded transition-colors"
                  @click="openFollowList('follower')"
                >
                  粉丝
                  <span class="text-foreground font-semibold">
                    {{ user.follower_count ?? '—' }}
                  </span>
                </button>
                <button
                  type="button"
                  class="hover:text-primary inline-flex items-center gap-1 rounded transition-colors"
                  @click="openFollowList('following')"
                >
                  关注
                  <span class="text-foreground font-semibold">
                    {{ user.following_count ?? '—' }}
                  </span>
                </button>
              </div>
            </div>
          </div>
          <p
            v-if="user.bio"
            class="text-default-600 mt-4 break-words whitespace-pre-line"
          >
            {{ user.bio }}
          </p>
          <KunContent
            v-if="user.about_html"
            class="mt-4"
            compact
            :content="user.about_html"
          />
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
              <KunTooltip
                v-if="user.is_followed"
                :text="notifyAll ? 'TA 发布新内容时通知我' : '只在「关注动态」中显示'"
              >
                <KunButton
                  is-icon-only
                  variant="flat"
                  color="primary"
                  :loading="notifyLoading"
                  :aria-label="notifyAll ? '关闭发布通知' : '开启发布通知'"
                  @click="toggleNotify"
                >
                  <KunIcon
                    :name="notifyAll ? 'lucide:bell-ring' : 'lucide:bell-off'"
                    class="size-4"
                  />
                </KunButton>
              </KunTooltip>
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
              <ReportButton
                subject-kind="user"
                :subject-id="userId"
                label="举报用户"
              />
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
                <div class="text-default-500 text-xs">发布 Galgame</div>
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

      <div class="min-w-0 lg:col-span-2">
        <KunTab
          v-model="currentTab"
          :items="tabs.map((t) => ({ value: t.key, textValue: t.title, href: t.href }))"
          variant="underlined"
          color="primary"
          size="md"
          class="mb-4"
        />
        <NuxtPage />
      </div>
    </div>
    <UserFollowListModal
      v-if="user"
      v-model:open="followOpen"
      :user-id="user.id"
      :mode="followMode"
      @follow-changed="refresh()"
    />
  </div>
  <KunNull v-else description="用户不存在" />
</template>
