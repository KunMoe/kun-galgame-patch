<script setup lang="ts">
const route = useRoute()
const api = useApi()

const VERDICT_NOT_FOUND = 40400

const { data } = await useAsyncData(
  () => `official-redirect-${route.params.id}`,
  async () => {
    const res = await api.get<{ catalog_id: number }>(
      `/taxonomy/resolve/official/${route.params.id}`
    )
    return { code: res.code, catalogId: res.data?.catalog_id ?? 0 }
  }
)

const answerFor = (code: number) =>
  code === VERDICT_NOT_FOUND
    ? kunPageError(404, '会社不存在')
    : kunPageError(502, '会社解析服务暂时不可用')

const verdict = data.value
if (verdict?.code === 0 && verdict.catalogId > 0) {
  await navigateTo(
    `/galgame/official/${verdict.catalogId}${route.query.page ? `?page=${route.query.page}` : ''}`,
    { redirectCode: 301 }
  )
} else {
  throw answerFor(verdict?.code ?? -1)
}
</script>

<template>
  <div class="container mx-auto my-6">
    <KunNull description="正在跳转到新的会社页面" />
  </div>
</template>
