<script setup lang="ts">
definePageMeta({ key: 'messages' })

if (!useRuntimeConfig().public.chatEnabled) {
  throw createError({ statusCode: 404, statusMessage: 'Page Not Found' })
}

useKunDisableSeo('私信')

const route = useRoute()
const userStore = useUserStore()
const chat = useChatStore()
const act = useChatActions()
const { open: openAuthModal } = useAuthModal()

const openId = computed(() =>
  typeof route.params.id === 'string' && route.params.id
    ? route.params.id
    : null
)
watch(openId, (id) => (chat.openId = id), { immediate: true })
onBeforeUnmount(() => (chat.openId = null))

const peer = computed(() => Number(route.query.peer) || 0)
watch(
  () => [chat.status, peer.value] as const,
  async ([status, uid]) => {
    if (status !== 'ready' || !uid) return
    const r = await act.openDirect(uid)
    if (r.code !== 0) useKunMessage(r.message || '无法发起私信', 'warn')
    await navigateTo(r.code === 0 ? `/messages/${r.data.id}` : '/messages', {
      replace: true
    })
  },
  { immediate: true }
)

const relogin = () => startOAuthLogin({ returnTo: route.fullPath })
</script>

<template>
  <div class="w-full">
    <div v-if="!userStore.isLoggedIn" class="mt-10 space-y-3 text-center">
      <KunNull description="登录后即可查看你的私信" />
      <KunButton color="primary" @click="openAuthModal()">登录</KunButton>
    </div>
    <ClientOnly v-else>
      <div v-if="chat.status === 'scope'" class="mt-10 space-y-3 text-center">
        <KunNull description="这次登录还没有授权私信，重新登录一次即可使用" />
        <KunButton color="primary" @click="relogin">重新登录</KunButton>
      </div>
      <KunNull
        v-else-if="chat.status === 'down'"
        class="mt-10"
        description="私信服务暂不可用，请稍后再试"
      />
      <div
        v-else-if="chat.status === 'ready'"
        class="border-default-200 rounded-kun-md bg-content1 h-[calc(100dvh-6rem)] overflow-hidden border"
      >
        <KunChatLayout :show-conversation="!!openId">
          <template #sidebar>
            <MessagesSidebar :open-id="openId" />
          </template>
          <MessagesPane v-if="openId" :id="openId" />
          <template #empty>
            <div
              class="bg-default-100 text-default-500 flex h-full items-center justify-center text-sm"
            >
              选择一个对话开始聊天
            </div>
          </template>
        </KunChatLayout>
      </div>
      <div v-else class="mt-10 flex justify-center">
        <KunLoading />
      </div>
    </ClientOnly>
  </div>
</template>
