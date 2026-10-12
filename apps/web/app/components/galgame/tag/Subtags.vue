<script setup lang="ts">
defineProps<{ subtags: TagSubtag[]; descendantCount: number }>()
</script>

<template>
  <section v-if="subtags.length" class="space-y-2">
    <h2 class="text-default-700 flex items-baseline gap-2 text-sm font-medium">
      子标签
      <span class="text-default-400 text-xs font-normal">
        直属 {{ subtags.length }} 个<template
          v-if="descendantCount > subtags.length"
          >，连同更细分的共 {{ descendantCount }} 个</template
        >
      </span>
    </h2>

    <KunScrollShadow axis="vertical" scrollbar="thin" class-name="max-h-48">
      <div class="flex flex-wrap gap-1.5">
        <KunLink
          v-for="sub in subtags"
          :key="sub.id"
          :to="`/galgame/tag/${sub.id}`"
          underline="none"
        >
          <KunChip
            size="sm"
            variant="flat"
            :color="sub.is_sexual ? 'danger' : 'default'"
            class-name="cursor-pointer"
          >
            {{ sub.name }}
            <span class="text-xs tabular-nums opacity-60">
              {{ sub.work_count }}
            </span>
            <span v-if="sub.descendant_count" class="text-xs opacity-60">
              · {{ sub.descendant_count }} 个子标签
            </span>
          </KunChip>
        </KunLink>
      </div>
    </KunScrollShadow>
  </section>
</template>
