<script setup lang="ts">
import {
  KUN_CONTENT_LIMIT_LABEL,
  KUN_CONTENT_LIMIT_MAP,
  KUN_CONTENT_LIMIT_OPTIONS
} from '~/constants/top-bar'
import type { KunNsfwStance } from '~/stores/settingStore'

const { stance, setStance, pending } = useKunNsfwStance()

const isDanger = computed(() => stance.value === 'show')

const onSelect = (key: KunNsfwStance) => setStance(key)
</script>

<template>
  <KunPopover position="bottom-end" inner-class="p-1 min-w-64">
    <template #trigger>
      <KunTooltip text="内容显示切换" position="bottom">
        <KunButton
          size="sm"
          variant="flat"
          :color="
            isDanger ? 'danger' : stance === 'blur' ? 'warning' : 'success'
          "
          aria-label="内容限制"
        >
          {{ KUN_CONTENT_LIMIT_LABEL[stance] }}
        </KunButton>
      </KunTooltip>
    </template>

    <div class="flex flex-col">
      <KunButton
        v-for="opt in KUN_CONTENT_LIMIT_OPTIONS"
        :key="opt.key"
        :variant="stance === opt.key ? 'flat' : 'light'"
        :color="stance === opt.key ? 'primary' : 'default'"
        size="sm"
        full-width
        rounded="md"
        class-name="justify-start"
        :disabled="pending"
        @click="onSelect(opt.key)"
      >
        <KunIcon :name="opt.icon" class="size-5" />
        {{ KUN_CONTENT_LIMIT_MAP[opt.key] }}
      </KunButton>
    </div>
  </KunPopover>
</template>
