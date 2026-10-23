<script setup lang="ts">
const props = defineProps<{
  resource: Pick<
    PatchResource,
    'archive_comment' | 'awaiting_password' | 'password'
  >
}>()

const URL_RE = /(https?:\/\/[^\s<>"'，。；）】」]+)/

const segments = computed(() =>
  (props.resource.archive_comment ?? '')
    .split(URL_RE)
    .filter(Boolean)
    .map((text) => ({ text, url: URL_RE.test(text) }))
)

const awaitingTitle = computed(() =>
  props.resource.password
    ? '作者填写的解压密码无法解开此文件'
    : '文件已加密，作者尚未提供解压密码'
)
</script>

<template>
  <div
    v-if="props.resource.awaiting_password || props.resource.archive_comment"
    class="space-y-2"
  >
    <KunInfo
      v-if="props.resource.awaiting_password"
      color="warning"
      variant="flat"
      :title="awaitingTitle"
      description="下载后可能无法解压，可以先看看备注或压缩包注释里有没有写密码。"
    />

    <KunAccordion v-if="props.resource.archive_comment" variant="bordered">
      <KunAccordionItem
        value="archive-comment"
        title="压缩包注释"
        icon="lucide:message-square-text"
      >
        <p class="text-default-700 text-sm break-all whitespace-pre-wrap">
          <template v-for="(s, i) in segments" :key="i">
            <KunLink
              v-if="s.url"
              :href="s.text"
              target="_blank"
              underline="hover"
              >{{ s.text }}</KunLink
            >
            <template v-else>{{ s.text }}</template>
          </template>
        </p>
      </KunAccordionItem>
    </KunAccordion>
  </div>
</template>
