<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

// Desktop top-bar ad button (ported 1:1 from the legacy next-web
// components/kun/ad/AIEroNav.tsx). The ad is shown to everyone EXCEPT ad-free
// roles (any non-"user" role: creator / moderator / admin). Legacy gated on
// `!uid || role < 2`; isAdFree is the OAuth-era equivalent that also covers
// creator (which isModerator did not). isAdFree reads the cookie-persisted
// roles during SSR, so the gate is correct on the first server render — no
// anonymous→hidden flicker.
const userStore = useUserStore()
</script>

<template>
  <KunTooltip v-if="!userStore.isAdFree" text="为什么现在的 AI 比人还要 H">
    <a :href="kunMoyuMoe.ad[0]?.url" target="_blank" rel="noopener noreferrer">
      <KunImage
        src="/a/moyumoe1-button.avif"
        alt=""
        provider="none"
        :skeleton="false"
        :width="500"
        :height="193"
        class-name="h-10 w-auto dark:opacity-80"
      />
    </a>
  </KunTooltip>
</template>
