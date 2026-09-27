<script setup lang="ts">
import { followingGroupPhrase, followingSiteLabel } from '~/utils/following'

const props = defineProps<{ group: FollowingActivityGroup }>()
const emit = defineEmits<{ vanish: [] }>()

const api = useApi()

const expanded = ref<FollowingActivityItem[] | null>(null)
const nextCursor = ref('')
const loading = ref(false)

const items = computed(() => expanded.value ?? props.group.items)
const hasHidden = computed(
  () =>
    expanded.value === null && props.group.item_count > props.group.items.length
)
const phrase = computed(() => followingGroupPhrase(props.group))
const siteLabel = computed(() => followingSiteLabel(props.group.site))

const loadItems = async () => {
  if (loading.value) return
  loading.value = true
  const query = new URLSearchParams({ limit: '20' })
  if (nextCursor.value) query.set('cursor', nextCursor.value)
  const res = await api.get<FollowingActivityItemPage>(
    `/community/following/group/${props.group.id}?${query}`
  )
  loading.value = false
  // The author hid their activities, or removed every item, after the page loaded.
  if (res.code === 40400) {
    emit('vanish')
    return
  }
  if (res.code !== 0) {
    useKunMessage(res.message || '加载失败，请稍后再试', 'error')
    return
  }
  expanded.value = [...(expanded.value ?? []), ...res.data.items]
  nextCursor.value = res.data.next_cursor
}
</script>

<template>
  <KunCard :bordered="true">
    <div class="space-y-3">
      <div class="flex items-center gap-3">
        <KunAvatar v-if="group.actor" :user="toKunUser(group.actor)" />
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
            <NuxtLink
              v-if="group.actor"
              :to="`/user/${group.actor.id}/resource`"
              class="hover:text-primary truncate font-semibold"
            >
              {{ group.actor.name }}
            </NuxtLink>
            <span v-else class="font-semibold">未知用户</span>
            <span class="text-default-600 text-sm">{{ phrase }}</span>
            <KunChip
              v-if="siteLabel"
              size="sm"
              variant="flat"
              color="secondary"
            >
              {{ siteLabel }}
            </KunChip>
          </div>
          <p class="text-default-400 text-xs">
            {{ formatDistanceToNow(group.latest_at) }}
          </p>
        </div>
      </div>

      <div class="space-y-3">
        <FollowingItem v-for="item in items" :key="item.id" :item="item" />
      </div>

      <KunButton
        v-if="hasHidden || nextCursor"
        variant="light"
        size="sm"
        :loading="loading"
        @click="loadItems"
      >
        {{ hasHidden ? `查看全部 ${group.item_count} 条` : '加载更多' }}
      </KunButton>
    </div>
  </KunCard>
</template>
