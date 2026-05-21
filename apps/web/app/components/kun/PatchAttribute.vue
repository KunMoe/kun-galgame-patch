<script setup lang="ts">
import {
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM_MAP,
  SUPPORTED_TYPE_MAP,
  SUPPORTED_RESOURCE_LINK_MAP
} from '~/constants/resource'
import type { KunUISize } from '@kun/ui/app/components/kun/ui/type'

interface Props {
  types: string[]
  languages: string[]
  platforms: string[]
  modelName?: string
  downloadCount?: number
  storage?: string
  storageSize?: string
  className?: string
  size?: KunUISize
}

const props = withDefaults(defineProps<Props>(), {
  modelName: '',
  downloadCount: undefined,
  storage: '',
  storageSize: '',
  className: '',
  size: 'md'
})
</script>

<template>
  <div :class="cn('flex flex-wrap gap-2', props.className)">
    <KunChip
      v-for="type in props.types"
      :key="type"
      variant="flat"
      color="primary"
      :size="props.size"
    >
      {{ SUPPORTED_TYPE_MAP[type] }}
    </KunChip>

    <KunChip
      v-for="lang in props.languages"
      :key="lang"
      variant="flat"
      color="secondary"
      :size="props.size"
    >
      {{ SUPPORTED_LANGUAGE_MAP[lang] }}
    </KunChip>

    <KunChip
      v-for="platform in props.platforms"
      :key="platform"
      variant="flat"
      color="success"
      :size="props.size"
    >
      {{ SUPPORTED_PLATFORM_MAP[platform] }}
    </KunChip>

    <KunChip
      v-if="props.modelName"
      variant="flat"
      color="danger"
      :size="props.size"
    >
      {{ props.modelName }}
    </KunChip>

    <KunChip v-if="props.storage" variant="flat" color="secondary">
      <KunIcon
        v-if="props.storage === 's3'"
        name="lucide:cloud"
        class="size-4"
      />
      <KunIcon
        v-else-if="props.storage === 'user'"
        name="lucide:link"
        class="size-4"
      />
      {{ SUPPORTED_RESOURCE_LINK_MAP[props.storage] }}
    </KunChip>

    <KunChip v-if="props.storageSize" variant="flat" color="warning">
      <KunIcon name="lucide:database" class="size-4" />
      {{ props.storageSize }}
    </KunChip>

    <KunChip
      v-if="props.downloadCount"
      variant="flat"
      color="default"
      :size="props.size"
    >
      {{ `${props.downloadCount} 人下载` }}
    </KunChip>
  </div>
</template>
