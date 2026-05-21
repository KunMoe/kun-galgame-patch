<script setup lang="ts">
interface Props {
  patch: GalgameCard
}

const props = defineProps<Props>()

const galgameName = computed(() => getPreferredLanguageText(props.patch.name))

const bannerSrc = computed(() => resolveBannerUrl(props.patch, 'mini'))
</script>

<template>
  <KunCard
    :href="`/patch/${props.patch.id}/introduction`"
    class-name="w-full"
  >
    <div class="flex flex-col gap-4 p-4 sm:flex-row">
      <div class="relative w-full sm:w-40">
        <KunImage
          :src="bannerSrc"
          :alt="galgameName"
          class-name="size-full rounded-lg object-cover"
        />
      </div>
      <div class="flex-1 space-y-3">
        <h2
          class="hover:text-primary-500 text-lg font-semibold transition-colors line-clamp-2"
        >
          {{ galgameName }}
        </h2>

        <KunCardStats :patch="props.patch" />
      </div>
    </div>
  </KunCard>
</template>
