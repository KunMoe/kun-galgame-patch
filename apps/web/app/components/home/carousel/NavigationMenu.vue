<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

const menuItems = [
  { href: '/doc', label: '文档', icon: 'lucide:notebook-pen' },
  { href: '/friend-link', label: '友情链接', icon: 'lucide:link' },
  {
    href: kunMoyuMoe.domain.kungal,
    label: '论坛',
    icon: 'lucide:message-square'
  },
  {
    href: kunMoyuMoe.domain.telegram_group,
    label: 'Telegram 群组',
    icon: 'fa6-brands:telegram'
  },
  {
    href: kunMoyuMoe.domain.cluster,
    label: '鲲 Galgame 网站集群',
    icon: 'lucide:server'
  }
]
</script>

<template>
  <div
    class="absolute inset-0 top-4 flex items-start justify-center gap-4 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
  >
    <KunTooltip v-for="item in menuItems" :key="item.label" :text="item.label">
      <KunLink
        :to="item.href"
        target="_blank"
        underline="none"
        color="default"
      >
        <KunButton
          is-icon-only
          color="default"
          variant="solid"
          :aria-label="item.label"
          class-name="bg-background/80 backdrop-blur-md transition-transform hover:scale-110"
        >
          <KunIcon :name="item.icon" class="size-5" />
        </KunButton>
      </KunLink>
    </KunTooltip>
  </div>
</template>
