<script setup lang="ts">
useKunDisableSeo('私聊')

const api = useApi()

const { data: rooms, pending } = await useAsyncData<ChatRoomSummary[]>(
  'chat-rooms',
  async () => {
    const res = await api.get<ChatRoomSummary[]>('/chat/room')
    return res.code === 0 ? res.data : []
  },
  { default: () => [] }
)
</script>

<template>
  <div class="space-y-4">
    <KunHeader name="私聊" description="您的私聊与群聊列表" />

    <KunLoading v-if="pending" description="正在加载聊天室..." />
    <template v-else-if="rooms && rooms.length">
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
            <span class="font-medium truncate">{{ room.name }}</span>
          </div>
          <p class="text-default-500 text-xs truncate">
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
    </template>

    <KunCard v-else :bordered="true">
      <div class="space-y-4 p-4 text-center">
        <KunIcon
          name="lucide:message-square-dashed"
          class="text-primary mx-auto size-12"
        />
        <h2 class="text-xl font-bold">欢迎来到聊天室</h2>
        <p class="text-default-500">
          在这里您可以与本站的任何用户私聊, 也可以创建/加入群组进行交流
        </p>
        <div class="text-default-500 text-sm">
          消息系统通过 HTTP 轮询实现, 简单可靠. 请勿发送重要消息
        </div>
      </div>
    </KunCard>
  </div>
</template>
