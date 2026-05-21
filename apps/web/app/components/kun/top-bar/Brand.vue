<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'
</script>

<template>
  <KunTooltip position="bottom">
    <template #content>
      <div class="space-y-1 px-1 py-2">
        <div class="text-small font-bold">如何记住本站的域名?</div>
        <div class="text-tiny text-foreground/80">
          鲲喜欢摸鱼 -&gt; 摸鱼(moyu) . 萌(moe)
        </div>
        <div class="text-tiny text-foreground/60">
          当然您也可以 Ctrl + D 收藏本站网址
        </div>
      </div>
    </template>

    <NuxtLink class="hidden items-center md:flex" to="/">
      <KunImage
        src="/favicon.webp"
        :alt="kunMoyuMoe.titleShort"
        :width="50"
        :height="50"
        class-name="shrink-0 rounded-2xl"
      />
      <p class="mr-2 ml-4 hidden font-bold lg:block">
        {{ kunMoyuMoe.creator.name }}
      </p>
      <KunChip
        class="hidden lg:flex"
        size="sm"
        variant="flat"
        color="primary"
      >
        补丁
      </KunChip>
    </NuxtLink>
  </KunTooltip>
</template>
