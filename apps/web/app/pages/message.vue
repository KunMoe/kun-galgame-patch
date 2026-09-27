<script setup lang="ts">
useKunDisableSeo('消息中心')

const route = useRoute()

const navItems = [
  { key: 'notice', title: '通知消息', href: '/message/notice', icon: 'lucide:bell' },
  { key: 'follow', title: '关注消息', href: '/message/follow', icon: 'lucide:users' },
  { key: 'mention', title: '@ 消息', href: '/message/mention', icon: 'lucide:at-sign' },
  {
    key: 'comment',
    title: '关注的评论区',
    href: '/message/comment',
    icon: 'lucide:message-square'
  },
  {
    key: 'patch-resource-create',
    title: '新补丁通知',
    href: '/message/patch-resource-create',
    icon: 'lucide:plus-circle'
  },
  {
    key: 'patch-resource-update',
    title: '补丁更新通知',
    href: '/message/patch-resource-update',
    icon: 'lucide:refresh-cw'
  },
  { key: 'system', title: '系统消息', href: '/message/system', icon: 'lucide:monitor-cog' },
  ...(useRuntimeConfig().public.chatEnabled
    ? [
        { key: 'messages', title: '私信', href: '/messages', icon: 'lucide:mail' },
        { key: 'chat', title: '群聊', href: '/message/chat', icon: 'lucide:users' }
      ]
    : [{ key: 'chat', title: '私聊', href: '/message/chat', icon: 'lucide:mail' }])
]

const currentKey = computed(
  () => route.path.split('/').filter(Boolean)[1] ?? ''
)
</script>

<template>
  <div class="container mx-auto my-4">
    <nav
      class="-mx-1 mb-4 flex gap-1 overflow-x-auto px-1 pb-1 lg:hidden"
      aria-label="消息分类"
    >
      <KunNavItem
        v-for="item in navItems"
        :key="item.key"
        :href="item.href"
        :label="item.title"
        :icon="item.icon"
        :current="currentKey === item.key"
        class-name="w-auto shrink-0 whitespace-nowrap"
      />
    </nav>

    <div class="grid gap-4 lg:grid-cols-4">
      <aside class="hidden lg:col-span-1 lg:block">
        <KunCard :bordered="true">
          <nav class="flex flex-col gap-1" aria-label="消息分类">
            <KunNavItem
              v-for="item in navItems"
              :key="item.key"
              :href="item.href"
              :label="item.title"
              :icon="item.icon"
              :current="currentKey === item.key"
            />
          </nav>
        </KunCard>
      </aside>

      <div class="lg:col-span-3">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
