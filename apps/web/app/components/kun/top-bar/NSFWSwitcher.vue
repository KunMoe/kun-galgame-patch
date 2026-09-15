<script setup lang="ts">
import {
  KUN_CONTENT_LIMIT_LABEL,
  KUN_CONTENT_LIMIT_MAP,
  KUN_CONTENT_LIMIT_OPTIONS
} from '~/constants/top-bar'
import type { KunNsfwPreference } from '~/stores/settingStore'

const settingStore = useSettingStore()

const isDanger = computed(() => {
  const v = settingStore.data.kunNsfwEnable
  return !!v && v !== 'sfw'
})

const onSelect = (key: KunNsfwPreference) => {
  settingStore.setNsfwPreference(key)
  // Persist synchronously before the reload: the plugin's own write rides a
  // pre-flush watcher (a microtask), and location.reload() would race it.
  settingStore.$persist()
  if (import.meta.client) location.reload()
}
</script>

<template>
  <KunPopover position="bottom-end" inner-class="p-1 min-w-64">
    <template #trigger>
      <KunTooltip text="内容显示切换" position="bottom">
        <KunButton
          size="sm"
          variant="flat"
          :color="isDanger ? 'danger' : 'success'"
          aria-label="内容限制"
        >
          {{ KUN_CONTENT_LIMIT_LABEL[settingStore.data.kunNsfwEnable] }}
        </KunButton>
      </KunTooltip>
    </template>

    <div class="flex flex-col">
      <KunButton
        v-for="opt in KUN_CONTENT_LIMIT_OPTIONS"
        :key="opt.key"
        :variant="settingStore.data.kunNsfwEnable === opt.key ? 'flat' : 'light'"
        :color="settingStore.data.kunNsfwEnable === opt.key ? 'primary' : 'default'"
        size="sm"
        full-width
        rounded="md"
        class-name="justify-start"
        @click="onSelect(opt.key)"
      >
        <KunIcon :name="opt.icon" class="size-5" />
        {{ KUN_CONTENT_LIMIT_MAP[opt.key] }}
      </KunButton>
    </div>
  </KunPopover>
</template>
