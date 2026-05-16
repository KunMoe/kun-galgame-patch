<script setup lang="ts">
interface Props {
  frontmatter: KunPostFrontmatter
}

const props = defineProps<Props>()
</script>

<template>
  <header class="mb-8 space-y-4">
    <div
      v-if="props.frontmatter.banner"
      class="bg-default-100 aspect-[3/1] w-full overflow-hidden rounded-xl"
    >
      <KunImage
        :src="props.frontmatter.banner"
        :alt="props.frontmatter.title"
        class-name="h-full w-full object-cover"
      />
    </div>

    <h1 class="text-3xl font-bold sm:text-4xl">
      {{ props.frontmatter.title }}
    </h1>

    <p v-if="props.frontmatter.description" class="text-default-500 text-lg">
      {{ props.frontmatter.description }}
    </p>

    <div class="text-default-500 flex items-center gap-3 text-sm">
      <img
        v-if="props.frontmatter.author_avatar"
        :src="props.frontmatter.author_avatar"
        :alt="props.frontmatter.author_name"
        class="size-8 rounded-full"
      />
      <span>{{ props.frontmatter.author_name }}</span>
      <span v-if="props.frontmatter.date">
        · {{ formatDate(props.frontmatter.date, { isShowYear: true }) }}
      </span>
    </div>

    <hr class="border-default/20" />
  </header>
</template>
