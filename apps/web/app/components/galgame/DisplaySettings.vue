<script setup lang="ts">
// "显示设置" — galgame card display preferences, sibling to the /galgame
// 高级筛选 button. Persists into the cookie-backed setting store (see
// stores/settingStore.ts), so GalgameCard reads the same values everywhere and
// SSR renders the chosen title language on first paint.
//
// `?? default` on every getter: an older cookie (written before these keys
// existed) won't carry them, so guard each read so the defaults still apply.
const settingStore = useSettingStore()

const open = ref(false)

const titleLanguage = computed({
  get: () => settingStore.data.titleLanguage ?? 'zh-cn',
  set: (v: 'zh-cn' | 'ja-jp') => settingStore.setData({ titleLanguage: v })
})
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
// Not a card-render toggle like the others: useApi forwards this as the global
// `include_empty` query param, so every moyu galgame list (home / galgame /
// ranking / a user's patches / favorites / contributions) hides resource-less
// games by default and includes them when this is on.
const showGalgamesWithoutResource = computed({
  get: () => settingStore.data.showGalgamesWithoutResource ?? false,
  set: (v: boolean) => settingStore.setData({ showGalgamesWithoutResource: v })
})

const titleLanguageOptions = [
  { value: 'zh-cn' as const, label: '中文' },
  { value: 'ja-jp' as const, label: '日语' }
]
</script>

<template>
  <button
    type="button"
    class="text-default-500 hover:text-primary flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors"
    @click="open = true"
  >
    <KunIcon name="lucide:eye" class="text-inherit" />
    <span>显示设置</span>
  </button>

  <KunModal v-model="open" inner-class-name="max-w-md">
    <div class="space-y-5 py-2">
      <h2 class="text-lg font-bold">显示设置</h2>

      <div class="space-y-2">
        <p class="text-sm font-medium">游戏标题优先语言</p>
        <KunRadioGroup
          v-model="titleLanguage"
          orientation="horizontal"
          :options="titleLanguageOptions"
        />
      </div>

      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium">显示日语副标题</p>
          <p class="text-default-500 text-xs">在标题下方显示游戏的日语标题</p>
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
          <p class="text-default-500 text-xs">在卡片上显示全年龄 / R18 标识</p>
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
  </KunModal>
</template>
