<script setup lang="ts">
import type { ArchiveNode } from '~/shared/utils/archiveTree'

const props = defineProps<{ file: ArchiveNode; src: string }>()

const open = defineModel<boolean>({ required: true })

const failed = ref(false)
watch(
  () => props.src,
  () => (failed.value = false)
)
</script>

<template>
  <KunModal v-model="open" :title="file.name" size="md">
    <div class="flex flex-col gap-3">
      <p class="text-default-400 truncate text-xs" :title="file.path">
        {{ file.parent || '/' }} · {{ formatBytes(file.size) }}
      </p>
      <audio
        :key="src"
        :src="src"
        controls
        preload="metadata"
        class="w-full"
        @error="failed = true"
      />
      <p v-if="failed" class="text-danger text-xs">
        无法播放这个音频：可能是浏览器不支持它的编码，或者打开得太频繁，请稍后再试。
      </p>
    </div>
  </KunModal>
</template>
