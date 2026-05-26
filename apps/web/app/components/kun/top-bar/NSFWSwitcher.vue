<script setup lang="ts">
import {
  KUN_CONTENT_LIMIT_LABEL,
  KUN_CONTENT_LIMIT_MAP
} from '~/constants/top-bar'
import type { KunNsfwPreference } from '~/stores/settingStore'

const settingStore = useSettingStore()

const options = [
  { key: 'sfw', icon: 'lucide:shield-check' },
  { key: 'nsfw', icon: 'lucide:ban' },
  { key: 'all', icon: 'lucide:circle-slash' }
] as const satisfies ReadonlyArray<{ key: KunNsfwPreference; icon: string }>

const isDanger = computed(() => {
  const v = settingStore.data.kunNsfwEnable
  return !!v && v !== 'sfw'
})

// onSelect takes the narrowed KunNsfwPreference (not raw string) so the
// store mutation type-checks against the recently-tightened store schema.
// location.reload() is intentional: every useApi composable captures the
// content_limit at setup time, so an in-place store update would only take
// effect on the *next* page navigation. A hard reload guarantees the
// switch takes effect immediately on the current page.
const onSelect = (key: KunNsfwPreference) => {
  settingStore.setNsfwPreference(key)
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
        v-for="opt in options"
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
