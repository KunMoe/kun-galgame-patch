<script setup lang="ts">
useKunSeoMeta({
  title: 'BLAKE3 文件校验工具',
  description:
    '在线 BLAKE3 文件哈希校验工具：上传文件即可计算 BLAKE3 哈希并与下载站给出的哈希比对，快速验证 Galgame 补丁文件是否完整无损、未被篡改。'
})
</script>

<template>
  <div class="my-4 w-full">
    <KunHeader
      name="BLAKE3 文件校验"
      description="您可以输入文件的 Hash, 然后上传文件以快速使用 BLAKE3 算法检查文件完好性"
    />
    <div class="mt-6">
      <CheckHashContainer />
    </div>
  </div>
</template>
