<script setup lang="ts">
// Home-link header: logo + name + optional sub-label (e.g. "论坛" /
// "Wiki" / "OAuth"). All values come from props so the component is
// usable across every consuming app without project-specific defaults.

withDefaults(
  defineProps<{
    name: string
    iconSrc?: string
    iconAlt?: string
    iconClass?: string
    badge?: string
    badgeColor?: import('./ui/type').KunUIColor
    to?: string
    nameClass?: string
  }>(),
  {
    iconSrc: '/favicon.webp',
    iconAlt: 'logo',
    iconClass: 'size-10 rounded-2xl',
    badge: '',
    badgeColor: 'primary',
    to: '/',
    nameClass: 'text-xl'
  }
)
</script>

<template>
  <div class="flex cursor-pointer items-center gap-3" @click="navigateTo(to)">
    <KunImage
      :class="iconClass"
      :src="iconSrc"
      :alt="iconAlt"
    />
    <span :class="nameClass">{{ name }}</span>
    <KunChip v-if="badge" size="md" :color="badgeColor">{{ badge }}</KunChip>
  </div>
</template>
