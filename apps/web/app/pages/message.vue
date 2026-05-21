<script setup lang="ts">
const route = useRoute()

const navItems = [
  { key: 'notice', title: '通知消息', href: '/message/notice', icon: 'lucide:bell' },
  { key: 'follow', title: '关注消息', href: '/message/follow', icon: 'lucide:users' },
  { key: 'mention', title: '@ 消息', href: '/message/mention', icon: 'lucide:at-sign' },
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
  { key: 'chat', title: '私聊', href: '/message/chat', icon: 'lucide:mail' }
]

// Writable computed for KunTab v-model — setter is a no-op since item.href
// drives the navigation via navigateTo(), and the route change re-runs
// the getter to update the active indicator.
const currentKey = computed({
  get: () => route.path.split('/').filter(Boolean)[1] ?? '',
  set: () => {}
})
</script>

<template>
  <div class="container mx-auto my-4">
    <div class="grid gap-4 lg:grid-cols-4">
      <aside class="lg:col-span-1">
        <KunCard :bordered="true">
          <KunTab
            v-model="currentKey"
            :items="navItems.map((n) => ({
              value: n.key,
              textValue: n.title,
              icon: n.icon,
              href: n.href
            }))"
            variant="light"
            color="primary"
            size="md"
            orientation="vertical"
          />
        </KunCard>
      </aside>

      <div class="lg:col-span-3">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
