<script setup lang="ts">
useKunDisableSeo('私聊')

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const { data: rooms, pending } = await useAsyncData<ChatRoomSummary[]>(
  'chat-rooms',
  async () => {
    const res = await api.get<ChatRoomSummary[]>('/chat/room')
    return res.code === 0 ? res.data : []
  },
  { default: () => [] }
)

// 加入测试群组 — the site provides one public group whose link is "kun"
// (restored from the legacy next-web ChatHint / CreateGroupChatModal default).
// JoinRoomByLink is idempotent (AddMember = ON CONFLICT DO NOTHING), so this is
// safe to click whether or not the user is already a member; then we open it.
const joining = ref(false)
const joinTestGroup = async () => {
  if (!requireLogin()) return
  joining.value = true
  try {
    const res = await api.post('/chat/room/join', { link: 'kun' })
    if (res.code === 0) {
      await navigateTo('/message/chat/kun')
    } else {
      useKunMessage(res.message || '加入群组失败', 'error')
    }
  } finally {
    joining.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <KunHeader name="私聊" description="您的私聊与群聊列表" />

    <KunLoading v-if="pending" description="正在加载聊天室..." />

    <template v-else>
      <!-- Existing rooms -->
      <NuxtLink
        v-for="room in rooms"
        :key="room.link"
        :to="`/message/chat/${room.link}`"
        class="border-default/20 bg-background hover:bg-default-100 flex items-center gap-3 rounded-lg border p-3 transition-colors"
      >
        <KunImage
          v-if="room.avatar"
          :src="room.avatar"
          :alt="room.name"
          class-name="bg-default-100 size-12 shrink-0 rounded-full"
        />
        <div
          v-else
          class="bg-default-100 flex size-12 shrink-0 items-center justify-center rounded-full"
        >
          <KunIcon
            :name="room.type === 'PRIVATE' ? 'lucide:user' : 'lucide:users'"
            class="text-default-500 size-5"
          />
        </div>
        <div class="flex-1 overflow-hidden">
          <div class="flex items-center gap-2">
            <KunIcon
              :name="room.type === 'PRIVATE' ? 'lucide:user' : 'lucide:users'"
              class="text-default-400 size-3.5"
            />
            <span class="truncate font-medium">{{ room.name }}</span>
          </div>
          <p class="text-default-500 truncate text-xs">
            {{ room.last_message || '暂无消息' }}
          </p>
        </div>
        <span
          v-if="room.last_message_time"
          class="text-default-400 shrink-0 text-xs"
        >
          {{ formatDistanceToNow(room.last_message_time) }}
        </span>
      </NuxtLink>

      <!-- Welcome + help hint (restored from the legacy next-web ChatHint). -->
      <KunCard :bordered="true">
        <div class="flex flex-col items-center gap-6 p-4 text-center md:p-6">
          <div class="flex flex-col items-center gap-2">
            <KunIcon
              name="lucide:message-square-dashed"
              class="text-primary size-12"
            />
            <h2 class="text-xl font-bold">欢迎来到我们的聊天室</h2>
            <p class="text-default-500 text-sm">
              在这里, 您可以与本站的任何用户私聊, 也可以创建 / 加入群组进行交流
            </p>
          </div>

          <div class="w-full space-y-4 text-left md:w-4/5">
            <div class="flex items-start gap-3">
              <KunIcon
                name="lucide:party-popper"
                class="text-warning mt-0.5 size-5 shrink-0"
              />
              <div>
                <p class="text-default-700 font-semibold">新功能尝鲜</p>
                <p class="text-default-500 text-sm">
                  聊天系统刚刚上线，我们正在努力完善它，难免会有一些 BUG
                </p>
              </div>
            </div>

            <div class="flex items-start gap-3">
              <KunIcon
                name="lucide:message-circle-question"
                class="text-secondary mt-0.5 size-5 shrink-0"
              />
              <div>
                <p class="text-default-700 font-semibold">期待您的反馈</p>
                <p class="text-default-500 text-sm">
                  如果碰到任何问题，请在
                  <a
                    href="https://www.kungal.com/topic/1820"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-primary font-semibold hover:underline"
                  >
                    我们的论坛
                  </a>
                  中告诉我们
                </p>
              </div>
            </div>

            <div class="flex items-start gap-3">
              <KunIcon
                name="lucide:github"
                class="text-default-600 mt-0.5 size-5 shrink-0"
              />
              <div>
                <p class="text-default-700 font-semibold">为我们点亮 Star</p>
                <p class="text-default-500 text-sm">
                  网站完全开源，如果喜欢，请给我们的
                  <a
                    href="https://github.com/KunMoe/kun-galgame-patch"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-primary font-semibold hover:underline"
                  >
                    GitHub 仓库
                  </a>
                  一颗 Star 以示支持！
                </p>
              </div>
            </div>

            <div class="flex items-start gap-3">
              <KunIcon
                name="lucide:triangle-alert"
                class="text-danger mt-0.5 size-5 shrink-0"
              />
              <div>
                <p class="text-default-700 font-semibold">注意事项</p>
                <p class="text-default-500 text-sm">
                  消息系统正在开发中, 请勿发送重要的消息, 以免造成消息丢失,
                  当消息系统正式稳定后将不会显示此提示
                </p>
              </div>
            </div>

            <div class="flex items-start gap-3">
              <KunIcon
                name="lucide:lightbulb"
                class="text-success mt-0.5 size-5 shrink-0"
              />
              <div>
                <p class="text-default-700 font-semibold">帮助信息</p>
                <p class="text-default-500 text-sm">
                  群聊的在线人数是精确的, 有人在的话就聊天吧! 设计灵感来源于
                  Telegram, 您可以加入我们的
                  <a
                    href="https://t.me/kungalgame"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-primary font-semibold hover:underline"
                  >
                    Telegram 群组
                  </a>
                </p>
              </div>
            </div>
          </div>

          <!-- 加入测试群组: the public group "kun" — "已经为大家提供了一个
               网站的公共群组, 快点击加入来聊天吧~" -->
          <div class="w-full space-y-2 md:w-4/5">
            <p class="text-default-500 text-sm">
              我们已经为大家提供了一个网站的公共测试群组, 快点击加入来聊天吧~
            </p>
            <KunButton
              color="primary"
              variant="solid"
              size="lg"
              full-width
              :loading="joining"
              :disabled="joining"
              @click="joinTestGroup"
            >
              <KunIcon name="lucide:plus" class="size-5" />
              加入测试群组
            </KunButton>
          </div>
        </div>
      </KunCard>
    </template>
  </div>
</template>
