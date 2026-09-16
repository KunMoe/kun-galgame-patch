<script setup lang="ts">
const props = defineProps<{
  subscription: CommentWallState | null
}>()

const emit = defineEmits<{
  setLevel: [level: CommentNotificationLevel]
}>()

const userStore = useUserStore()
const pending = ref(false)

const subscribed = computed(() => props.subscription?.subscribed ?? false)

// Unfollow is level 1 (normal), not 0 (muted): muted also silences replies
// and mentions addressed to the reader.
const toggle = async () => {
  pending.value = true
  emit('setLevel', subscribed.value ? 1 : 3)
  pending.value = false
}
</script>

<template>
  <KunButton
    v-if="userStore.user.id"
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
