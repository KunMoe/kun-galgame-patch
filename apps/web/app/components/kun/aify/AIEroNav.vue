<script setup lang="ts">
const userStore = useUserStore()
const aifyUrl = useRuntimeConfig().public.umamiAifyUrl
</script>

<template>
  <span
    v-if="!userStore.isAifyFree && aifyUrl"
    :class="cn(userStore.isLoggedIn && 'hidden md:block')"
  >
    <KunTooltip text="为什么现在的 AI 比人还要 H">
      <a :href="aifyUrl" target="_blank" rel="noopener noreferrer">
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
  </span>
</template>
