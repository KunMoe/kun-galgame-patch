<script setup lang="ts">
const props = defineProps<{
  series: PatchDetailSeries[]
}>()
</script>

<template>
  <section v-if="series.length" class="space-y-4">
    <KunHeader
      name="Galgame 系列"
      description="该 Galgame 所属的系列作品合集"
      scale="h2"
    />

    <div class="flex flex-wrap gap-2">
      <NuxtLink
        v-for="s in series"
        :key="s.id"
        :to="`/galgame/series/${s.id}`"
      >
        <KunChip color="primary" variant="flat" size="sm">
          {{ s.name }}
        </KunChip>
      </NuxtLink>
    </div>
  </section>
</template>
