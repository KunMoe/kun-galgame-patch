<script setup lang="ts">
// System (display) preferences. Local-only, cookie-backed via settingStore so
// SSR renders the chosen values on first paint.
//   - 主题: relocated here from the top bar (the top-bar theme button is gone).
//   - 内容显示 (NSFW): a COPY of the top-bar / mobile switcher (those stay).
//   - Galgame 卡片显示设置: moved from the /galgame "显示设置" modal; the
//     /galgame entry now links to this card (#galgame-display).
import { KUN_CONTENT_LIMIT_MAP } from '~/constants/top-bar'
import type { KunNsfwPreference } from '~/stores/settingStore'

useKunDisableSeo('系统设置')

const route = useRoute()
const settingStore = useSettingStore()

// ── 主题 ──────────────────────────────────────────────
const colorMode = useColorMode()
const themes = [
  { key: 'light', label: '浅色主题', icon: 'lucide:sun' },
  { key: 'dark', label: '深色主题', icon: 'lucide:moon' },
  { key: 'system', label: '跟随系统', icon: 'lucide:sun-moon' }
] as const
const setTheme = (key: 'light' | 'dark' | 'system') => {
  colorMode.preference = key
}

// ── 内容显示 (NSFW) ───────────────────────────────────
// location.reload() on change for the same reason as the top-bar switcher:
// useApi captures content_limit at setup, so the change only applies on the
// next navigation — a reload makes it take effect on the current page.
const nsfwOptions = [
  { key: 'sfw', icon: 'lucide:shield-check' },
  { key: 'all', icon: 'lucide:circle-slash' },
  { key: 'nsfw', icon: 'lucide:ban' }
] as const satisfies ReadonlyArray<{ key: KunNsfwPreference; icon: string }>
const setNsfw = (key: KunNsfwPreference) => {
  settingStore.setNsfwPreference(key)
  if (import.meta.client) location.reload()
}

// ── Galgame 卡片显示设置 (moved from the /galgame modal) ──
const titleLanguage = computed({
  get: () => settingStore.data.titleLanguage ?? 'ja-jp',
  set: (v: 'zh-cn' | 'ja-jp') => settingStore.setData({ titleLanguage: v })
})
const titleLanguageOptions = [
  { value: 'zh-cn' as const, label: '中文' },
  { value: 'ja-jp' as const, label: '日语' }
]
const showJapaneseSubtitle = computed({
  get: () => settingStore.data.showJapaneseSubtitle ?? false,
  set: (v: boolean) => settingStore.setData({ showJapaneseSubtitle: v })
})
const showReleaseDate = computed({
  get: () => settingStore.data.showReleaseDate ?? false,
  set: (v: boolean) => settingStore.setData({ showReleaseDate: v })
})
const showNsfwBadge = computed({
  get: () => settingStore.data.showNsfwBadge ?? true,
  set: (v: boolean) => settingStore.setData({ showNsfwBadge: v })
})
const showGalgamesWithoutResource = computed({
  get: () => settingStore.data.showGalgamesWithoutResource ?? false,
  set: (v: boolean) => settingStore.setData({ showGalgamesWithoutResource: v })
})

// Smooth-scroll to the section the /galgame "显示设置" link deep-links to.
onMounted(() => {
  if (route.hash) {
    nextTick(() => {
      document
        .querySelector(route.hash)
        ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  }
})
</script>

<template>
  <div class="w-full">
    <div class="max-w-3xl space-y-6">
      <!-- 主题 -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">主题</h2>
        </template>
        <div class="space-y-2">
          <p class="text-default-500 text-xs">选择网站的明暗主题</p>
          <div
            class="border-default/20 bg-default-50/40 grid grid-cols-3 gap-1 rounded-xl border p-1"
          >
            <KunButton
              v-for="t in themes"
              :key="t.key"
              :variant="colorMode.preference === t.key ? 'flat' : 'light'"
              :color="colorMode.preference === t.key ? 'primary' : 'default'"
              size="sm"
              full-width
              rounded="lg"
              class-name="flex-col gap-1 py-3"
              :aria-label="`切换到${t.label}`"
              @click="setTheme(t.key)"
            >
              <KunIcon :name="t.icon" class="size-5" />
              <span class="text-xs">{{ t.label }}</span>
            </KunButton>
          </div>
        </div>
      </KunCard>

      <!-- 内容显示 (NSFW) -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">内容显示</h2>
        </template>
        <div class="space-y-2">
          <p class="text-default-500 text-xs">
            控制是否显示 R18 等成人内容（切换后会刷新页面以立即生效）
          </p>
          <div
            class="border-default/20 bg-default-50/40 grid grid-cols-3 gap-1 rounded-xl border p-1"
          >
            <KunButton
              v-for="opt in nsfwOptions"
              :key="opt.key"
              :variant="
                settingStore.data.kunNsfwEnable === opt.key ? 'flat' : 'light'
              "
              :color="
                settingStore.data.kunNsfwEnable === opt.key
                  ? 'primary'
                  : 'default'
              "
              size="sm"
              full-width
              rounded="lg"
              class-name="flex-col gap-1 py-3"
              :aria-label="`切换内容模式: ${KUN_CONTENT_LIMIT_MAP[opt.key]}`"
              @click="setNsfw(opt.key)"
            >
              <KunIcon :name="opt.icon" class="size-5" />
              <span class="text-xs">{{ KUN_CONTENT_LIMIT_MAP[opt.key] }}</span>
            </KunButton>
          </div>
        </div>
      </KunCard>

      <!-- Galgame 卡片显示设置 — deep-link target from /galgame 显示设置. -->
      <div id="galgame-display" class="scroll-mt-24">
        <KunCard :bordered="true">
          <template #header>
            <h2 class="px-1 pt-1 text-xl font-medium">Galgame 卡片显示设置</h2>
          </template>
          <div class="space-y-5">
            <div class="space-y-2">
              <p class="text-sm font-medium">游戏标题优先语言</p>
              <p class="text-default-500 text-xs">
                站内所有页面的游戏名都会优先使用所选语言显示（缺失时回退到其它语言）
              </p>
              <KunRadioGroup
                v-model="titleLanguage"
                orientation="horizontal"
                :options="titleLanguageOptions"
              />
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <p class="text-sm font-medium">显示日语副标题</p>
                <p class="text-default-500 text-xs">
                  在标题下方显示游戏的日语标题
                </p>
              </div>
              <KunSwitch v-model="showJapaneseSubtitle" />
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <p class="text-sm font-medium">显示发售时间</p>
                <p class="text-default-500 text-xs">在卡片上显示游戏的发售日期</p>
              </div>
              <KunSwitch v-model="showReleaseDate" />
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <p class="text-sm font-medium">显示 NSFW 状态</p>
                <p class="text-default-500 text-xs">
                  在卡片上显示全年龄 / R18 标识
                </p>
              </div>
              <KunSwitch v-model="showNsfwBadge" />
            </div>

            <div class="flex items-center justify-between gap-4">
              <div>
                <p class="text-sm font-medium">显示无补丁资源的游戏</p>
                <p class="text-default-500 text-xs">
                  列表中包含暂时没有补丁资源的 Galgame
                </p>
              </div>
              <KunSwitch v-model="showGalgamesWithoutResource" />
            </div>
          </div>
        </KunCard>
      </div>
    </div>
  </div>
</template>
