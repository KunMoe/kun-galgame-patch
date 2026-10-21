<script setup lang="ts">
defineOptions({ name: 'galgame-library' })

useKunSeoMeta({
  title: '鲲 Galgame 论坛数据',
  description:
    'Galgame 信息资料数据库，收录全部 Galgame 作品资料，包含本站暂无补丁资源的作品。可按热度、发售日期、资料更新时间排序，并按发售年份筛选，打开任意作品即可发布补丁。'
})
</script>

<template>
  <GalgameBrowser mode="library" />
</template>
