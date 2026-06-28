<script setup lang="ts">
// keepalive: returning from a card restores the home feed + scroll position
// instead of remounting. Safe — static route with a static useAsyncData key.
definePageMeta({ keepalive: true })

const api = useApi()

const { data } = await useAsyncData<HomeResponse>(
  'home',
  async () => {
    const response = await api.get<HomeResponse>('/home')
    if (response.code !== 0) {
      return { galgames: [], resources: [], comments: [] }
    }
    return response.data
  },
  {
    default: () => ({ galgames: [], resources: [], comments: [] })
  }
)

useKunSeoMeta({
  title: '首页',
  description:
    '开源, 免费, 零门槛, 纯手写, 最先进的 Galgame 补丁资源下载站, 提供 Windows, 安卓, KRKR, Tyranor 等各类平台的 Galgame 补丁资源下载。永远免费！'
})
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <HomeContainer
      :galgames="data?.galgames ?? []"
      :resources="data?.resources ?? []"
      :comments="data?.comments ?? []"
    />
  </div>
</template>
