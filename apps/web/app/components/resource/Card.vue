<script setup lang="ts">
interface Props {
  resource: PatchResource
}

const props = defineProps<Props>()

// Global resource rows now carry a `patch` summary (id/name/banner) — fall
// back to the resource's own name when the row is rendered in a context where
// the patch wasn't enriched (e.g. user profile page).
const galgameName = computed(() =>
  props.resource.patch?.name
    ? getPreferredLanguageText(props.resource.patch.name)
    : ''
)
const title = computed(
  () =>
    galgameName.value ||
    props.resource.name ||
    props.resource.note ||
    '补丁资源'
)

const userDescription = computed(() => {
  const when = formatDate(props.resource.created, {
    isShowYear: true,
    isPrecise: true
  })
  return `发布于 ${when}`
})
</script>

<template>
  <KunCard
    :href="`/resource/${props.resource.id}`"
    class-name="w-full"
  >
    <div class="flex flex-col justify-between space-y-2">
      <div class="flex">
        <!-- is-navigation=false: this whole card is already a <KunCard :href>
             link to the resource. Since 0.12.0 KunUserChip renders a real <a>
             to the publisher when navigable, which would nest <a> inside <a>.
             Keep the card the single link; the chip stays plain. -->
        <KunUserChip
          :user="props.resource.user"
          :description="userDescription"
          :is-navigation="false"
        />
      </div>

      <h2
        class="hover:text-primary-500 text-lg font-semibold transition-colors line-clamp-2"
      >
        {{ title }}
      </h2>

      <KunPatchAttribute
        :types="props.resource.type"
        :languages="props.resource.language"
        :platforms="props.resource.platform"
        :model-name="props.resource.model_name"
        size="sm"
      />

      <div
        class="text-small text-default-500 flex items-center justify-between"
      >
        <div class="flex gap-4">
          <div class="flex items-center gap-1">
            <KunIcon name="lucide:heart" class="size-4" />
            {{ props.resource.like_count }}
          </div>
          <div class="flex items-center gap-1">
            <KunIcon name="lucide:download" class="size-4" />
            {{ props.resource.download }}
          </div>
        </div>
        <KunChip size="sm" variant="flat">
          {{ props.resource.size }}
        </KunChip>
      </div>
    </div>
  </KunCard>
</template>
