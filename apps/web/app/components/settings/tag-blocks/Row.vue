<script setup lang="ts">
const props = defineProps<{
  id: number
  item?: SearchEntityItem
  mode: KunTagBlockMode
  busy: boolean
}>()

const emit = defineEmits<{ unblock: []; mode: [mode: KunTagBlockMode] }>()

const expanded = ref(false)

const name = computed(() =>
  props.item ? getPreferredLanguageText(props.item.name) : `#${props.id}`
)
const descendants = computed(() => props.item?.descendant_count ?? 0)
const modeOptions = computed(() => [
  { value: 'tree' as const, label: `含 ${descendants.value} 个子标签` },
  { value: 'exact' as const, label: '仅此标签' }
])
</script>

<template>
  <div class="border-default-200 space-y-2 rounded-lg border px-3 py-2">
    <div class="flex items-center justify-between gap-3">
      <KunLink
        :to="`/galgame/tag/${id}`"
        :color="item?.is_sexual ? 'danger' : 'default'"
        underline="hover"
        class-name="min-w-0 truncate text-sm font-medium"
      >
        {{ name }}
      </KunLink>
      <KunTooltip text="取消屏蔽" position="left">
        <KunButton
          size="xs"
          variant="light"
          is-icon-only
          :disabled="busy"
          aria-label="取消屏蔽"
          @click="emit('unblock')"
        >
          <KunIcon name="lucide:x" />
        </KunButton>
      </KunTooltip>
    </div>

    <div
      v-if="descendants"
      class="flex flex-wrap items-center justify-between gap-2"
    >
      <KunRadioGroup
        :model-value="mode"
        :options="modeOptions"
        variant="pill"
        orientation="horizontal"
        size="xs"
        :disabled="busy"
        :aria-label="`${name} 的屏蔽范围`"
        class-name="w-auto"
        @update:model-value="(next: KunTagBlockMode) => emit('mode', next)"
      />
      <KunButton size="xs" variant="light" @click="expanded = !expanded">
        {{ expanded ? '收起' : '查看子标签' }}
        <KunIcon
          :name="expanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
          class="size-3"
        />
      </KunButton>
    </div>
    <p v-else-if="item" class="text-default-400 text-xs">没有子标签</p>

    <GalgameTagBlockDescendants v-if="expanded && descendants" :tag-id="id" />
  </div>
</template>
