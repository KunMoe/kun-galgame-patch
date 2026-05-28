<script setup lang="ts">
// Message center shell + every nested message/* page disables SEO —
// these surfaces are per-user inboxes, no public content.
useKunDisableSeo('消息中心')

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

// Active category = 2nd path segment (/message/<key>[/...]). Matching the
// segment rather than the exact path keeps 私聊 lit on a transcript page
// (/message/chat/<link>), not just the chat index.
const currentKey = computed(
  () => route.path.split('/').filter(Boolean)[1] ?? ''
)

// Returns a :class array (no `cn` dependency in <script>). Shared by both the
// mobile strip and the desktop sidebar so the active styling stays identical.
const navLinkClass = (key: string) => [
  'flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors',
  currentKey.value === key
    ? 'bg-primary text-white'
    : 'text-default-600 hover:bg-default-100'
]
</script>

<template>
  <div class="container mx-auto my-4">
    <!-- Mobile (<lg): horizontal scrollable strip on top. The desktop sidebar
         would otherwise collapse into a 7-item vertical wall above the
         content on a single-column layout. -->
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
      </NuxtLink>
    </nav>

    <div class="grid gap-4 lg:grid-cols-4">
      <!-- Desktop (lg+): persistent vertical sidebar. -->
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
