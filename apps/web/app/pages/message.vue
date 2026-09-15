<script setup lang="ts">
useKunDisableSeo('消息中心')

const route = useRoute()
const messageStore = useMessageStore()
const { refresh: refreshUnread } = useUnreadCounts()

const navItems = [
  { key: 'notice', title: '通知消息', href: '/message/notice', icon: 'lucide:bell', countKey: 'notice' },
  { key: 'follow', title: '关注消息', href: '/message/follow', icon: 'lucide:users', countKey: 'follow' },
  { key: 'mention', title: '@ 消息', href: '/message/mention', icon: 'lucide:at-sign', countKey: 'mention' },
  {
    key: 'patch-resource-create',
    title: '新补丁通知',
    href: '/message/patch-resource-create',
    icon: 'lucide:plus-circle',
    countKey: 'patchResourceCreate'
  },
  {
    key: 'patch-resource-update',
    title: '补丁更新通知',
    href: '/message/patch-resource-update',
    icon: 'lucide:refresh-cw',
    countKey: 'patchResourceUpdate'
  },
  { key: 'system', title: '系统消息', href: '/message/system', icon: 'lucide:monitor-cog', countKey: 'system' },
  { key: 'chat', title: '私聊', href: '/message/chat', icon: 'lucide:mail', countKey: 'chat' }
]

const countOf = (key: string) => messageStore.unreadCounts[key] ?? 0

const currentKey = computed(
  () => route.path.split('/').filter(Boolean)[1] ?? ''
)

const navLinkClass = (key: string) => [
  'flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors',
  currentKey.value === key
    ? 'bg-primary text-white'
    : 'text-default-600 hover:bg-default-100'
]

onMounted(() => refreshUnread())
</script>

<template>
  <div class="container mx-auto my-4">
    <nav
      class="-mx-1 mb-4 flex gap-1 overflow-x-auto px-1 pb-1 lg:hidden"
      aria-label="消息分类"
    >
      <NuxtLink
        v-for="item in navItems"
        :key="item.key"
        :to="item.href"
        :class="[navLinkClass(item.key), 'shrink-0 whitespace-nowrap']"
      >
        <KunIcon :name="item.icon" class="size-4 shrink-0" />
        {{ item.title }}
        <span v-if="countOf(item.countKey) > 0" class="ml-auto">
          <KunBadge
            variant="count"
            :count="countOf(item.countKey)"
            class-name="bg-primary-100 text-primary-700"
          />
        </span>
      </NuxtLink>
    </nav>

    <div class="grid gap-4 lg:grid-cols-4">
      <aside class="hidden lg:col-span-1 lg:block">
        <KunCard :bordered="true">
          <nav class="flex flex-col gap-1" aria-label="消息分类">
            <NuxtLink
              v-for="item in navItems"
              :key="item.key"
              :to="item.href"
              :class="navLinkClass(item.key)"
            >
              <KunIcon :name="item.icon" class="size-4 shrink-0" />
              {{ item.title }}
              <span v-if="countOf(item.countKey) > 0" class="ml-auto">
                <KunBadge
                  variant="count"
                  :count="countOf(item.countKey)"
                  class-name="bg-primary-100 text-primary-700"
                />
              </span>
            </NuxtLink>
          </nav>
        </KunCard>
      </aside>

      <div class="lg:col-span-3">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
