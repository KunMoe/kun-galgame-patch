<script setup lang="ts">
// System (display) preferences. Local-only, cookie-backed via settingStore so
// SSR renders the chosen language on first paint. The "游戏标题优先语言" option
// moved here from the /galgame "显示设置" modal because it now drives game-name
// language on EVERY page (via getPreferredLanguageText), not just the card.
useKunDisableSeo('系统设置')

const settingStore = useSettingStore()

const titleLanguage = computed({
  get: () => settingStore.data.titleLanguage ?? 'zh-cn',
  set: (v: 'zh-cn' | 'ja-jp') => settingStore.setData({ titleLanguage: v })
})

const titleLanguageOptions = [
  { value: 'zh-cn' as const, label: '中文' },
  { value: 'ja-jp' as const, label: '日语' }
]
</script>

<template>
  <div class="my-4 w-full">
    <KunHeader name="系统设置" description="本地显示偏好，保存在浏览器中，更换设备需重新设置" />

    <div class="mx-auto my-4 max-w-3xl space-y-6">
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">显示设置</h2>
        </template>
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
      </KunCard>
    </div>
  </div>
</template>
