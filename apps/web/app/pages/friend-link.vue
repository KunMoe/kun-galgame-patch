<script setup lang="ts">
import { kunFriends } from '~/config/friend'
import { kunMoyuMoe } from '~/config/moyu-moe'

useKunSeoMeta({
  title: '友情链接',
  description:
    '鲲 Galgame 补丁站的友情链接，汇集 Galgame 资讯、汉化补丁、轻小说翻译、二次元社区等同好站点。'
})

// Tag every outbound 友链 with our domain as utm_source so a友站 can see the
// traffic we send. URL API preserves any existing query params; a non-absolute
// or malformed link falls through untouched.
const utmSource = new URL(kunMoyuMoe.domain.main).hostname
const appendUtm = (link: string): string => {
  try {
    const url = new URL(link)
    url.searchParams.set('utm_source', utmSource)
    return url.toString()
  } catch {
    return link
  }
}
const friends = kunFriends.map((friend) => ({
  ...friend,
  link: appendUtm(friend.link)
}))
</script>

<template>
  <div class="container mx-auto my-8">
    <div>
      <h1 class="text-primary-500 mb-4 text-center text-4xl">友情链接</h1>
      <p class="text-default-500 mb-12 text-center">
        下方是我们的友站, 您可以点击以访问这些网站
      </p>
    </div>

    <div
      class="grid grid-cols-2 gap-2 sm:gap-6 md:grid-cols-3 lg:grid-cols-4"
    >
      <a
        v-for="friend in friends"
        :key="friend.name"
        :href="friend.link"
        target="_blank"
        rel="noopener noreferrer"
        class="border-default-200 bg-background hover:bg-default-100 block h-full w-full rounded-lg border p-4 transition-colors"
      >
        <div class="flex w-full justify-center pt-2">
          <KunImage
            :alt="friend.name"
            class-name="h-24 w-24 rounded-lg object-cover"
            :src="friend.avatar"
          />
        </div>
        <div class="flex flex-col items-center pt-4 pb-2">
          <h4 class="text-large font-bold">{{ friend.name }}</h4>
          <p class="text-default-500 mt-1 text-center text-sm line-clamp-4">
            {{ friend.label }}
          </p>
        </div>
      </a>
    </div>

    <div class="mt-16">
      <h2 class="text-default-800 mb-4 text-center text-2xl">加入我们</h2>
      <p class="text-default-500 mb-12 text-center">
        要加入我们, 请加入我们的
        <a
          :href="kunMoyuMoe.domain.telegram_group"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary inline-flex items-center gap-1 hover:underline"
        >
          Telegram 群组
          <KunIcon name="lucide:external-link" class="size-3.5" />
        </a>
        联系我们
      </p>
    </div>
  </div>
</template>
