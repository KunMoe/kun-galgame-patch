<script setup lang="ts">
import type { KunUserProps } from './type'

const props = withDefaults(defineProps<KunUserProps>(), {
  size: 'md',
  description: '',
  className: '',
  disableFloating: false,
  floatingPosition: 'top'
})
</script>

<template>
  <div :class="cn('flex items-center gap-2', props.className)">
    <KunAvatar
      :floating-position="props.floatingPosition"
      :disable-floating="props.disableFloating"
      :user="user"
      :size="size"
    />

    <div class="flex flex-col text-sm">
      <!-- user may be null/undefined when the OAuth /users/batch brief
           couldn't be resolved. Degrade gracefully instead of throwing
           "Cannot read properties of undefined (reading 'name')" and
           500-ing the page. KunAvatar already tolerates null. -->
      <span>{{ user?.name || '未知用户' }}</span>
      <span v-if="description" class="text-default-500">
        {{ description }}
      </span>
    </div>
  </div>
</template>
