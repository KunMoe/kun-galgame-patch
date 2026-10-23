<script setup lang="ts">

interface Props {
  resource: PatchResource
}

const props = defineProps<Props>()

const emit = defineEmits<{
  downloaded: []
}>()

const isResourceDisabled = computed(() => (props.resource.status ?? 0) !== 0)

const downloadLinks = computed(() =>
  resolveDownloadLinks(
    props.resource.storage,
    props.resource.content,
    props.resource.download_url
  )
)

const aria2CommandOf = (url: string) => `aria2c -x16 -s16 -c '${url}'`
</script>

<template>
  <div class="border-success/40 bg-success/10 space-y-4 rounded-2xl border p-5">
    <div
      v-if="isResourceDisabled"
      class="border-danger/30 bg-danger/10 text-danger-700 flex items-center gap-2 rounded-xl border p-3 text-sm"
    >
      <KunIcon name="lucide:shield-alert" class="size-4 shrink-0" />
      <span>
        该资源已被禁用下载（可能存在安全风险，或应发布者 / 版主要求下架），暂时无法获取下载链接。
      </span>
    </div>

    <p v-if="!isResourceDisabled" class="text-default-500 text-sm">
      点击下方链接下载（共 {{ downloadLinks.length }} 个）
    </p>

    <div v-if="!isResourceDisabled" class="space-y-2">
      <div
        v-for="(lnk, i) in downloadLinks"
        :key="i"
        class="border-success/40 bg-content1 shadow-kun-sm hover:border-success focus-within:border-success space-y-3 rounded-xl border p-3 transition-colors"
      >
        <a
          :href="lnk"
          target="_blank"
          rel="noopener noreferrer"
          class="hover:text-success group flex min-w-0 items-center gap-3 transition-colors"
          @click="emit('downloaded')"
        >
          <span
            class="bg-success/15 text-success flex size-9 shrink-0 items-center justify-center rounded-lg"
          >
            <KunIcon name="lucide:download" class="size-5" />
          </span>
          <span class="min-w-0 flex-1 truncate text-sm">{{ lnk }}</span>
          <KunIcon
            name="lucide:external-link"
            class="text-default-400 group-hover:text-success size-4 shrink-0"
          />
        </a>
        <div class="flex flex-wrap gap-2">
          <KunCopy
            :text="lnk"
            name="复制下载链接"
            copied-text="已复制链接"
            color="success"
            variant="flat"
            size="md"
          />
          <KunCopy
            :text="aria2CommandOf(lnk)"
            name="复制 aria2 命令"
            copied-text="已复制命令"
            color="success"
            variant="flat"
            size="md"
          />
        </div>
      </div>
      <p v-if="!downloadLinks.length" class="text-default-500 text-sm">
        暂无可用下载链接
      </p>
    </div>

    <div
      v-if="props.resource.code || props.resource.password"
      class="flex flex-wrap gap-2"
    >
      <KunCopy
        v-if="props.resource.code"
        :text="props.resource.code"
        :name="`提取码: ${props.resource.code}`"
        color="secondary"
        variant="flat"
        size="sm"
      />
      <KunCopy
        v-if="props.resource.password"
        :text="props.resource.password"
        :name="`解压密码: ${props.resource.password}`"
        color="secondary"
        variant="flat"
        size="sm"
      />
    </div>

    <ResourceArchiveInfo :resource="props.resource" />

    <div
      v-if="props.resource.blake3 && props.resource.storage !== 'user'"
      class="border-success/30 space-y-2 border-t pt-3"
    >
      <p class="text-default-500 text-xs">BLAKE3 校验码，可校验下载文件完整性</p>
      <div class="flex flex-wrap items-center gap-2">
        <code
          class="bg-background/60 max-w-full truncate rounded-lg px-2 py-1 text-xs"
        >
          {{ props.resource.blake3 }}
        </code>
        <KunCopy
          :text="props.resource.blake3"
          name="复制"
          size="sm"
          variant="flat"
        />
        <NuxtLink
          :to="`/check-hash?hash=${props.resource.blake3}&content=${encodeURIComponent(props.resource.content || '')}`"
        >
          <KunButton size="sm" variant="flat" color="primary" rounded="full">
            <KunIcon name="lucide:shield-check" class="size-3.5" />
            前往校验页面
          </KunButton>
        </NuxtLink>
      </div>
    </div>
  </div>
</template>
