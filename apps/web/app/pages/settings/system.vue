<script setup lang="ts">
import {
  KUN_CONTENT_LIMIT_RADIO_OPTIONS,
  KUN_THEME_OPTIONS
} from '~/constants/top-bar'

useKunDisableSeo('系统设置')

const route = useRoute()
const settingStore = useSettingStore()

const { theme, contentStance } = useKunDisplayPreference()

const galgameListLayout = computed({
  get: () => settingStore.data.galgameListLayout ?? 'poster',
  set: (v: 'poster' | 'row') => settingStore.setData({ galgameListLayout: v })
})
const layoutOptions = [
  { value: 'poster' as const, label: '封面网格' },
  { value: 'row' as const, label: '详细列表' }
]

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
  get: () => settingStore.data.showNsfwBadge ?? false,
  set: (v: boolean) => settingStore.setData({ showNsfwBadge: v })
})
const showGalgamesWithoutResource = computed({
  get: () => settingStore.data.showGalgamesWithoutResource ?? false,
  set: (v: boolean) => settingStore.setData({ showGalgamesWithoutResource: v })
})

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
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">外观与内容</h2>
        </template>
        <div class="space-y-5">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-medium">主题</p>
              <p class="text-default-500 text-xs">网站的明暗配色</p>
            </div>
            <KunRadioGroup
              v-model="theme"
              :options="KUN_THEME_OPTIONS"
              variant="pill"
              orientation="horizontal"
              size="sm"
              aria-label="主题"
              class-name="w-auto shrink-0"
            />
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-medium">内容显示</p>
              <p class="text-default-500 text-xs">
                R18 等成人内容的显示方式：隐藏 / 模糊 /
                直接显示。登录后该设置保存在 NextMoe 账号上，全站通用
              </p>
            </div>
            <KunRadioGroup
              v-model="contentStance"
              :options="KUN_CONTENT_LIMIT_RADIO_OPTIONS"
              variant="pill"
              orientation="horizontal"
              size="sm"
              aria-label="内容显示"
              class-name="w-auto shrink-0"
            />
          </div>
        </div>
      </KunCard>

      <div id="galgame-display" class="scroll-mt-24">
        <KunCard :bordered="true">
          <template #header>
            <h2 class="px-1 pt-1 text-xl font-medium">Galgame 卡片显示设置</h2>
          </template>
          <div class="space-y-5">
            <div class="space-y-2">
              <p class="text-sm font-medium">列表布局</p>
              <p class="text-default-500 text-xs">
                站内 Galgame
                列表的排布方式。封面网格一屏能看到更多作品；详细列表在封面右侧列出会社、发售日期、剧本、原画与标签
                (剧本 / 原画 / 标签 目前只在 Galgame
                补丁资源库与信息资料库两个页面提供)
              </p>
              <div
                class="border-default/20 bg-default-50/40 grid grid-cols-2 gap-1 rounded-xl border p-1"
              >
                <KunButton
                  v-for="opt in layoutOptions"
                  :key="opt.value"
                  :variant="galgameListLayout === opt.value ? 'flat' : 'light'"
                  :color="
                    galgameListLayout === opt.value ? 'primary' : 'default'
                  "
                  size="sm"
                  full-width
                  rounded="lg"
                  class-name="h-auto flex-col items-stretch gap-2 p-2"
                  :aria-label="`切换到${opt.label}`"
                  @click="galgameListLayout = opt.value"
                >
                  <GalgameLayoutPreview :layout="opt.value" />
                  <span class="text-xs">{{ opt.label }}</span>
                </KunButton>
              </div>
            </div>

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
                <p class="text-default-500 text-xs">
                  在卡片上显示游戏的发售日期
                </p>
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
