<script setup lang="ts">
import { callCommand } from '@milkdown/kit/utils'
import { insertImageCommand } from '@milkdown/kit/preset/commonmark'
import { commands } from './_buttonList'
import { tabs, activeTab } from '../../atom'
import { uploadEditorImage } from '../upload/uploader'
import type { UseEditorReturn } from '@milkdown/vue'
import type { CmdKey } from '@milkdown/kit/core'

const props = defineProps<{
  editorInfo: UseEditorReturn
  isShowUploadImage: boolean
}>()

const { get } = props.editorInfo
const input = ref<HTMLElement>()

const runtimeConfig = useRuntimeConfig()
const apiBase =
  (runtimeConfig.public.apiBase as string) || 'http://127.0.0.1:5214/api/v1'
const userStore = useUserStore()

const call = <T,>(command: CmdKey<T>, payload?: T, callback?: () => void) => {
  const result = get()?.action(callCommand(command, payload))
  if (callback) {
    callback()
  }
  return result
}

const handleFileChange = async (event: Event) => {
  const target = event.target as HTMLInputElement
  if (!target.files || !target.files[0]) {
    return
  }
  const file = target.files[0]

  useKunMessage('正在上传图片...', 'info')
  const src = await uploadEditorImage(apiBase, file)
  if (src) {
    const filename = file.name.replace(/[^a-zA-Z0-9 ]/g, '')
    const userName = userStore.user.name
    const imageName = `${userName}-${Date.now()}-${filename}`
    call(insertImageCommand.key, {
      src,
      title: imageName,
      alt: imageName
    })
    useKunMessage('图片上传成功', 'success')
  } else {
    useKunMessage('图片上传失败', 'error')
  }
}

const toggleTab = () => {
  activeTab.value = activeTab.value === 'preview' ? 'code' : 'preview'
}

const currentTabLabel = computed(() => {
  return activeTab.value === 'preview' ? tabs[1]!.textValue : tabs[0]!.textValue
})
</script>

<template>
  <div class="flex flex-wrap items-center space-x-1">
    <KunButton variant="light" @click="toggleTab">
      {{ currentTabLabel }}
    </KunButton>

    <template v-if="activeTab === 'preview'">
      <KunMilkdownPluginsHeader :editor-info="editorInfo" />

      <KunButton
        :is-icon-only="true"
        v-for="(btn, index) in commands"
        :key="index"
        class-name="text-xl"
        variant="light"
        @click="call(btn.command.key, btn.payload)"
      >
        <KunIcon class="text-foreground" :name="btn.icon" />
      </KunButton>

      <KunButton
        :is-icon-only="true"
        v-if="props.isShowUploadImage"
        variant="light"
        class-name="text-xl"
        @click="input?.click()"
      >
        <KunIcon class="text-foreground" name="lucide:image-plus" />
        <input
          hidden
          ref="input"
          type="file"
          accept=".jpg, .jpeg, .png, .webp"
          @change="handleFileChange($event)"
        />
      </KunButton>

      <KunPopover inner-class="-left-28">
        <template #trigger>
          <KunButton variant="light" class-name="text-xl" :is-icon-only="true">
            <KunIcon class="text-foreground" name="lucide:smile-plus" />
          </KunButton>
        </template>

        <KunMilkdownPluginsEmojiContainer :editor-info="editorInfo" />
      </KunPopover>

      <KunPopover inner-class="-left-28">
        <template #trigger>
          <KunButton variant="light" class-name="text-xl" :is-icon-only="true">
            <KunIcon class="text-foreground" name="lucide:sticker" />
          </KunButton>
        </template>

        <KunMilkdownPluginsStickerContainer :editor-info="editorInfo" />
      </KunPopover>
    </template>
  </div>
</template>
