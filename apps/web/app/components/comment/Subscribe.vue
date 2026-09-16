<script setup lang="ts">
const props = defineProps<{
  threadId: number
  subscription: CommentThreadState | null
}>()

const emit = defineEmits<{
  setLevel: [level: CommentNotificationLevel]
}>()

const userStore = useUserStore()
const pending = ref(false)

const subscribed = computed(() => props.subscription?.subscribed ?? false)

// The community service has four notification levels, but its unread listing
// only ever asks "is this muted?" — normal, tracking and watching all count the
// same. Offering four buttons for one distinction would be three lies.
const toggle = async () => {
  pending.value = true
  emit('setLevel', subscribed.value ? 0 : 3)
  pending.value = false
}
</script>

<template>
  <!--
    A wall nobody has commented on has no thread yet, and following a thread that
    does not exist is not something the community service can do — the first
    comment is what creates it.
  -->
  <KunButton
    v-if="threadId && userStore.user.id"
    variant="light"
    size="sm"
    :color="subscribed ? 'primary' : 'default'"
    :loading="pending"
    @click="toggle"
  >
    <KunIcon
      :name="subscribed ? 'lucide:bell' : 'lucide:bell-plus'"
      class="size-4"
    />
    {{ subscribed ? '已关注' : '关注评论区' }}
  </KunButton>
</template>
