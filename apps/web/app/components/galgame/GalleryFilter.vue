<script setup lang="ts">
// Per-rating gallery filter (galgame detail 画廊), ported from kungal. Two
// independent axes:
//   色情 (sexual)   — gated by the global NSFW mode: NSFW shows every level,
//                     SFW reveals only the levels opted-in here.
//   暴力 (violence) — ALWAYS an explicit per-level opt-in (default off), shown
//                     behind a prominent confirm. The NSFW mode does NOT unlock
//                     it (sex and gore are separate sensitivities).
// Each level shows how many images carry it. The actual hide/show runs in
// Gallery.vue; this only edits the persisted level sets (settingStore, cookie-
// persisted so the choice survives across pages / sessions).
const props = defineProps<{
  showNsfw: boolean
  hiddenCount: number
  // level (1/2/3) → number of screenshots carrying that rating
  sexualCounts: Record<number, number>
  violenceCounts: Record<number, number>
}>()

const settingStore = useSettingStore()
const sexualLevels = computed(() => settingStore.data.gallerySexualLevels ?? [])
const violenceLevels = computed(
  () => settingStore.data.galleryViolenceLevels ?? []
)

const LEVELS = [
  { value: 1, label: '轻' },
  { value: 2, label: '中' },
  { value: 3, label: '高' }
]

// Only offer levels that actually have images.
const sexualShown = computed(() =>
  LEVELS.filter((lv) => (props.sexualCounts[lv.value] ?? 0) > 0)
)
const violenceShown = computed(() =>
  LEVELS.filter((lv) => (props.violenceCounts[lv.value] ?? 0) > 0)
)

const toggleSexual = (level: number) =>
  settingStore.toggleGalleryLevel('sexual', level)

// Violence: turning it ON from the all-off state pops a confirm first; once any
// violence level is on (or persisted from a prior session), further toggles
// don't re-prompt.
const warnOpen = ref(false)
const pendingLevel = ref<number | null>(null)

const onViolence = (level: number) => {
  const enabling = !violenceLevels.value.includes(level)
  if (enabling && violenceLevels.value.length === 0) {
    pendingLevel.value = level
    warnOpen.value = true
    return
  }
  settingStore.toggleGalleryLevel('violence', level)
}

const confirmViolence = () => {
  if (pendingLevel.value !== null) {
    settingStore.toggleGalleryLevel('violence', pendingLevel.value)
  }
  pendingLevel.value = null
  warnOpen.value = false
}
</script>

<template>
  <KunPopover position="bottom-end" inner-class="w-72 p-3">
    <template #trigger>
      <KunButton variant="flat" size="sm">
        <KunIcon name="lucide:funnel" />
        分级筛选
        <KunChip v-if="hiddenCount" size="sm" color="warning" variant="flat">
          隐藏 {{ hiddenCount }}
        </KunChip>
      </KunButton>
    </template>

    <div class="space-y-4">
      <!-- 色情 -->
      <div class="space-y-2">
        <p class="text-default-700 text-sm font-medium">色情评级</p>
        <p v-if="!sexualShown.length" class="text-default-400 text-xs">
          无色情评级图片
        </p>
        <p v-else-if="showNsfw" class="text-default-500 text-xs">
          NSFW 模式已显示全部色情内容。
        </p>
        <div v-else class="flex flex-col gap-2">
          <KunCheckBox
            v-for="lv in sexualShown"
            :id="`gal-sexual-${lv.value}`"
            :key="lv.value"
            color="primary"
            :model-value="sexualLevels.includes(lv.value)"
            :label="`${lv.label} · ${sexualCounts[lv.value]} 张`"
            @update:model-value="() => toggleSexual(lv.value)"
          />
        </div>
      </div>

      <KunDivider />

      <!-- 暴力 -->
      <div class="space-y-2">
        <div class="flex items-center gap-1.5">
          <KunIcon name="lucide:triangle-alert" class="text-danger-500" />
          <p class="text-default-700 text-sm font-medium">暴力评级</p>
        </div>
        <p v-if="!violenceShown.length" class="text-default-400 text-xs">
          无暴力评级图片
        </p>
        <template v-else>
          <p class="text-danger-600 text-xs">
            暴力 / 血腥内容可能引起不适，默认隐藏，确认后才显示。
          </p>
          <div class="flex flex-col gap-2">
            <KunCheckBox
              v-for="lv in violenceShown"
              :id="`gal-violence-${lv.value}`"
              :key="lv.value"
              color="danger"
              :model-value="violenceLevels.includes(lv.value)"
              :label="`${lv.label} · ${violenceCounts[lv.value]} 张`"
              @update:model-value="() => onViolence(lv.value)"
            />
          </div>
        </template>
      </div>

      <KunDivider />
      <p class="text-default-400 text-xs leading-relaxed">
        缩略图描边：<span class="text-warning-500">外圈 = 色情</span> ·
        <span class="text-danger-500">内圈 = 暴力</span>，颜色越深级别越高。
      </p>
    </div>
  </KunPopover>

  <KunModal v-model="warnOpen" inner-class-name="max-w-sm">
    <div class="space-y-4 text-center">
      <KunIcon
        name="lucide:triangle-alert"
        class="text-danger-500 mx-auto text-4xl"
      />
      <div class="space-y-1">
        <p class="text-lg font-semibold">显示暴力内容？</p>
        <p class="text-default-600 text-sm">
          画廊中存在带有暴力 / 血腥评级的图片，可能引起不适。确认要显示这类内容吗？
        </p>
      </div>
      <div class="flex justify-center gap-2">
        <KunButton variant="flat" color="default" @click="warnOpen = false">
          取消
        </KunButton>
        <KunButton color="danger" @click="confirmViolence">确认显示</KunButton>
      </div>
    </div>
  </KunModal>
</template>
