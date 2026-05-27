<script setup lang="ts">
// One-line global banner shown ONLY when the site is in SFW mode (content
// is being hidden) — once the user opts into "显示全部内容" we drop the
// banner entirely so the open-state isn't constantly re-affirmed.
//
// Mounted once in the layout above KunTopBar — intentionally NOT sticky,
// so it scrolls away once the user starts browsing (TopBar takes over the
// top edge). The banner exists to make the gate state legible, not to nag.
//
// State source is settingStore.data.kunNsfwEnable (cookie-persisted via
// piniaPluginPersistedstate.cookies()), so SSR sees the same value as
// the client and the banner doesn't flicker on hydration. Crawlers (no
// cookie → default 'sfw') see the banner, which is also fine — it
// actively signals "this site has gated content" to whoever scrapes the
// SSR HTML.
const settingStore = useSettingStore()

const isSafeMode = computed(() => settingStore.data.kunNsfwEnable === 'sfw')
</script>

<template>
  <div
    v-if="isSafeMode"
    class="bg-danger/10 text-danger border-danger/20 w-full border-b px-3 py-1.5 text-center text-xs"
    role="status"
    aria-live="polite"
  >
    <KunIcon
      name="lucide:shield-check"
      class="mr-1 inline size-3.5 align-text-bottom"
    />
    当前为 SFW 模式，部分 R18 / NSFW 内容已隐藏 — 您可在右上角切换 "显示全部内容"
  </div>
</template>
