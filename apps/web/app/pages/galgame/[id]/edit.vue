<script setup lang="ts">
const route = useRoute()
const id = Number(route.params.id)

if (!Number.isInteger(id) || id < 1) {
  throw kunPageError(404, '没有找到这个游戏')
}

useKunDisableSeo('前往未萌编辑台')

await navigateTo(useNextmoeEditorLink()(id), {
  external: true,
  redirectCode: 302,
  replace: true
})
</script>

<template>
  <div class="py-16">
    <KunLoading description="正在前往未萌编辑台" />
  </div>
</template>
