<script setup lang="ts">
// The single, app-wide login/register modal. Mounted ONCE in the default layout
// and opened from anywhere via useAuthModal(). Body is the shared AuthEntry
// (login + register, both bounce to 鲲 Galgame OAuth) — identical to what the
// top-bar 登录 button used to show inline.
const { isOpen } = useAuthModal()
</script>

<template>
  <KunModal v-model="isOpen" inner-class-name="max-w-sm">
    <div class="flex flex-col items-center gap-6 py-4">
      <h2 class="text-2xl font-bold">登录或注册</h2>
      <AuthEntry />
    </div>
  </KunModal>
</template>
