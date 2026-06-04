<script setup lang="ts">
const currentYear = new Date().getFullYear()
</script>

<template>
  <footer class="border-default/20 mt-8 w-full border-t text-sm">
    <div class="mx-auto max-w-7xl px-4">
      <div
        class="flex flex-wrap justify-center gap-4 py-6 md:justify-between"
      >
        <NuxtLink to="/" class="flex items-center space-x-2">
          <KunImage
            src="/favicon.webp"
            alt="鲲 Galgame 补丁"
            :width="30"
            :height="30"
            class-name="rounded-full"
          />
          <span>© {{ currentYear }} 鲲 Galgame 补丁</span>
        </NuxtLink>

        <div class="flex space-x-8">
          <NuxtLink to="/about" class="flex items-center">使用指南</NuxtLink>
          <a
            href="https://nav.kungal.org"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center"
          >
            网站集群
          </a>
          <NuxtLink to="/friend-link" class="flex items-center">
            友情链接
          </NuxtLink>
          <a
            href="https://github.com/KunMoe/kun-galgame-patch-next"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center"
          >
            GitHub 仓库
          </a>
        </div>

        <div class="flex space-x-8">
          <span class="flex items-center">联系我们</span>
          <a
            href="https://t.me/kungalgame"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center"
          >
            Telegram
          </a>
          <a href="https://www.kungal.com" class="flex items-center">
            主站论坛
          </a>
        </div>
      </div>
    </div>
  </footer>
</template>
