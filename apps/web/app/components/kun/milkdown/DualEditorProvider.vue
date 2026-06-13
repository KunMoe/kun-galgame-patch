<script setup lang="ts">
import { MilkdownProvider } from '@milkdown/vue'
import { ProsemirrorAdapterProvider } from '@prosemirror-adapter/vue'
import { activeTab } from './atom'

withDefaults(
  defineProps<{
    valueMarkdown: string
    language?: Language
    // Pass false to strip every image affordance (upload button, sticker
    // picker, paste/drop upload) — used by the galgame intro editor. Defaults
    // true so comments / resource notes keep images.
    allowImage?: boolean
  }>(),
  { allowImage: true, language: 'zh-cn' }
)

const emits = defineEmits<{
  setMarkdown: [value: string]
}>()

const cmAPI = ref({
  update: (_: string) => {}
})

const saveMarkdown = (markdown: string) => {
  cmAPI.value.update(markdown)
  emits('setMarkdown', markdown)
}

const setCmAPI = (api: { update: (markdown: string) => void }) => {
  cmAPI.value = api
}
</script>

<template>
  <MilkdownProvider>
    <ProsemirrorAdapterProvider>
      <div class="space-y-3">
        <KunMilkdownEditor
          :value-markdown="valueMarkdown"
          @save-markdown="saveMarkdown"
          :language="language ?? 'zh-cn'"
          :allow-image="allowImage"
        >
          <template #footer>
            <slot />
          </template>
        </KunMilkdownEditor>

        <template v-if="activeTab === 'code'">
          <KunMilkdownCodemirror
            :markdown="valueMarkdown"
            @set-cm-api="setCmAPI"
            @on-change="(value) => emits('setMarkdown', value)"
          />
        </template>
      </div>
    </ProsemirrorAdapterProvider>
  </MilkdownProvider>
</template>
