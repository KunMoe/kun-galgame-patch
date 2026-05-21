<script setup lang="ts">
import { MESSAGE_TYPE_MAP, MESSAGE_TYPE_ICON } from '~/constants/message'

interface Props {
  msg: Message
}

const props = defineProps<Props>()

const cardHref = computed(() => {
  if (!props.msg.sender) return '/'
  if (!props.msg.link) return `/user/${props.msg.sender.id}/resource`
  return props.msg.link
})

const iconName = computed(
  () => MESSAGE_TYPE_ICON[props.msg.type] ?? 'lucide:bell'
)
</script>

<template>
  <NuxtLink
    :to="cardHref"
    class="border-default/20 bg-background hover:bg-default-100 block rounded-lg border p-4 transition-colors"
  >
    <div class="flex items-start gap-3">
      <KunAvatar
        v-if="props.msg.sender"
        :user="props.msg.sender"
        :is-navigation="false"
      />
      <img
        v-else
        src="/favicon.webp"
        alt="系统"
        class="size-8 rounded-full"
      />

      <div class="flex-1 space-y-1">
        <div class="flex flex-wrap items-center gap-2">
          <KunIcon :name="iconName" class="text-primary size-4" />
          <span class="font-semibold">
            {{ props.msg.sender ? props.msg.sender.name : '系统' }}
          </span>
          <span class="text-default-500 text-sm">
            {{ MESSAGE_TYPE_MAP[props.msg.type] ?? props.msg.type }}
          </span>
        </div>
        <p class="text-default-600 whitespace-pre-wrap">
          {{ props.msg.content }}
        </p>
        <span class="text-default-400 text-xs">
          {{ formatDistanceToNow(props.msg.created) }}
        </span>
      </div>

      <KunChip
        :color="props.msg.status === 0 ? 'danger' : 'default'"
        size="sm"
      >
        {{ props.msg.status === 0 ? '新消息' : '已阅读' }}
      </KunChip>
    </div>
  </NuxtLink>
</template>
