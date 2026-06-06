<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

// Mobile-menu ad button (ported from the legacy next-web
// components/kun/ad/AIEroMobileNav.tsx). Same audience gate as AIEroNav —
// hidden for moderators/admins. The desktop AIEroNav is `hidden md:block`, so
// this is the phone (< md) counterpart, rendered inside KunTopBarMobileMenu.
const userStore = useUserStore()
</script>

<template>
  <a
    v-if="!userStore.isModerator"
    :href="kunMoyuMoe.ad[0]?.url"
    target="_blank"
    rel="noopener noreferrer"
    class="flex items-center gap-2"
  >
    <KunImage
      src="/a/moyumoe1-button.avif"
      alt=""
      provider="none"
      :skeleton="false"
      :width="500"
      :height="193"
      class-name="h-11 w-auto dark:opacity-80"
    />
  </a>
</template>
