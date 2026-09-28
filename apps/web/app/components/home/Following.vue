<script setup lang="ts">
import { followingGroupPhrase, followingSiteLabel } from '~/utils/following'

const api = useApi()
const { unseen, check } = useFollowingUnseen()

const { data: groups } = await useAsyncData('home-following', async () => {
  const res = await api.get<FollowingActivityPage>(
    '/community/following?limit=6'
  )
  return res.code === 0 ? res.data.groups : null
})

onMounted(check)

const rows = computed(() =>
  (groups.value ?? []).map((group) => {
    const item = group.items[0]
    return {
      id: group.id,
      actor: group.actor,
      phrase: followingGroupPhrase(group),
      item: item && {
        title: item.title,
        to: item.path ?? item.url,
        target: item.path === null ? '_blank' : undefined
      },
      site: followingSiteLabel(group.site),
      time: formatDistanceToNow(group.latest_at)
    }
  })
)
</script>

<template>
  <KunCard
    :bordered="true"
    padding="md"
    content-class="flex h-full flex-col gap-3"
  >
    <div class="flex items-center gap-2">
      <h2 class="text-lg font-bold">关注动态</h2>
      <KunChip v-if="unseen > 0" size="sm" variant="flat" color="danger">
        {{ unseen }} 条新动态
      </KunChip>
      <KunButton
        variant="light"
        color="primary"
        size="sm"
        href="/following"
        class-name="ml-auto"
      >
        查看更多
        <KunIcon name="lucide:chevron-right" class="size-4" />
      </KunButton>
    </div>

    <KunNull v-if="!groups" description="关注动态加载失败，请稍后再试" />

    <div
      v-else-if="!rows.length"
      class="flex flex-1 flex-col items-center justify-center gap-2 text-center"
    >
      <KunNull description="你关注的人还没有动态" />
      <p class="text-default-500 text-sm">
        在用户主页点击「关注」，TA 的新动态会出现在这里
      </p>
    </div>

    <KunOverlayScroll v-else class="-mx-1 min-h-0 flex-1">
      <ul class="space-y-1">
        <li
          v-for="row in rows"
          :key="row.id"
          class="hover:bg-default-100 flex items-center gap-3 rounded-lg px-1 py-1.5"
        >
          <KunAvatar v-if="row.actor" :user="toKunUser(row.actor)" size="sm" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm">
              <span class="font-medium">{{
                row.actor?.name ?? '未知用户'
              }}</span>
              <span class="text-default-500 ml-1">{{ row.phrase }}</span>
              <span v-if="row.site" class="text-default-400 ml-1">
                · {{ row.site }}
              </span>
            </p>
            <NuxtLink
              v-if="row.item"
              :to="row.item.to"
              :target="row.item.target"
              class="text-default-700 hover:text-primary block truncate text-sm"
            >
              {{ row.item.title }}
            </NuxtLink>
          </div>
          <span class="text-default-400 shrink-0 text-xs">{{ row.time }}</span>
        </li>
      </ul>
    </KunOverlayScroll>
  </KunCard>
</template>
