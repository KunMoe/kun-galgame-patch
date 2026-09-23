<script setup lang="ts">
import { MESSAGE_TYPE_MAP, MESSAGE_TYPE_ICON } from '~/constants/message'

interface Props {
  msg: Message
}

const props = defineProps<Props>()
const router = useRouter()

// A system notice has no sender, and sending every one of those to `/` threw
// away the link each moderator notice carries. The route check is for the
// historic `/apply/success` links, whose page no longer exists.
const cardHref = computed(() => {
  const { link, sender } = props.msg
  if (link && router.resolve(link).matched.length) return link
  return sender ? `/user/${sender.id}/resource` : '/'
})

const iconName = computed(
  () => MESSAGE_TYPE_ICON[props.msg.type] ?? 'lucide:bell'
)

const gameName = computed(() => getPreferredLanguageText(props.msg.galgame_name))
const displayContent = computed(() => {
  const name = gameName.value
  if (!name) return props.msg.content
  switch (props.msg.type) {
    case 'favorite':
      return name
    case 'favoriteResource':
      return `收藏了您在 ${name} 下发布的补丁资源`
    case 'likeResource':
      return `点赞了您在 ${name} 下发布的补丁资源`
    default:
      return props.msg.content
  }
})
</script>

<template>
  <NuxtLink
    :to="cardHref"
    class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block rounded-lg border p-4 transition-colors"
  >
    <div class="flex items-start gap-3">
      <KunAvatar
        v-if="props.msg.sender"
        :user="toKunUser(props.msg.sender)"
        :is-navigation="false"
      />
      <KunImage
        v-else
        src="/favicon.webp"
        alt="系统"
        class-name="size-8 rounded-full"
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
          {{ displayContent }}
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
