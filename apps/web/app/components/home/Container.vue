<script setup lang="ts">
interface Props {
  galgames: GalgameCard[]
  resources: HomeResource[]
  comments: HomeComment[]
}

const props = defineProps<Props>()
</script>

<template>
  <div class="mx-auto max-w-7xl space-y-8">
    <HomeHero />

    <section class="space-y-3 sm:space-y-6">
      <div class="flex items-center space-x-4">
        <h2 class="text-2xl font-bold">最新 Galgame</h2>
        <KunButton variant="light" color="primary" href="/galgame">
          查看更多
          <KunIcon name="lucide:chevron-right" class="size-4" />
        </KunButton>
      </div>
      <div
        class="mx-auto mb-8 grid grid-cols-2 gap-2 sm:grid-cols-2 sm:gap-6 lg:grid-cols-3 xl:grid-cols-4"
      >
        <GalgameCard
          v-for="galgame in props.galgames"
          :key="galgame.id"
          :patch="galgame"
        />
      </div>
    </section>

    <section class="space-y-3 sm:space-y-6">
      <div class="flex items-center space-x-4">
        <h2 class="text-2xl font-bold">最新补丁资源下载</h2>
        <KunButton variant="light" color="primary" href="/resource">
          查看更多
          <KunIcon name="lucide:chevron-right" class="size-4" />
        </KunButton>
      </div>

      <div class="grid grid-cols-1 gap-3 sm:gap-6 md:grid-cols-2">
        <ResourceCard
          v-for="resource in props.resources"
          :key="resource.id"
          :resource="resource"
        />
      </div>
    </section>

    <section class="space-y-3 sm:space-y-6">
      <div class="flex items-center space-x-4">
        <h2 class="text-2xl font-bold">最新评论</h2>
        <KunButton variant="light" color="primary" href="/comment">
          查看更多
          <KunIcon name="lucide:chevron-right" class="size-4" />
        </KunButton>
      </div>

      <div class="space-y-4">
        <CommentCard
          v-for="comment in props.comments"
          :key="comment.id"
          :comment="comment"
        />
      </div>
    </section>
  </div>
</template>
