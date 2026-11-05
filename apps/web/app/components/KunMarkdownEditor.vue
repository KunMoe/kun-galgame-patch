<script setup lang="ts">
import type { KunEditorFeatures } from '@kungal/editor-core'
import type { KunToolbarItem } from '@kungal/editor-vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    image?: boolean
    locale?: string
    placeholder?: string
    toolbar?: KunToolbarItem[]
    features?: KunEditorFeatures
    readonly?: boolean
  }>(),
  {
    image: true,
    locale: 'zh-cn',
    placeholder: '',
    toolbar: () => [
      'heading',
      '|',
      'bold',
      'italic',
      'strike',
      'code',
      'link',
      '|',
      'bulletList',
      'orderedList',
      'quote',
      'codeBlock',
      'hr',
      '|',
      'spoiler',
      '|',
      'picker'
    ],
    features: () => ({}),
    readonly: false
  }
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const adapters = useKunEditorAdapters({ image: props.image })

const onUpdate = (value: string) => emit('update:modelValue', value)

const editorViews: ('wysiwyg' | 'source')[] = ['wysiwyg', 'source']
</script>

<template>
  <KunEditor
    :model-value="modelValue"
    :adapters="adapters"
    :features="features"
    :readonly="readonly"
    :locale="locale"
    :placeholder="placeholder"
    :views="editorViews"
    @update:model-value="onUpdate"
  >
    <template #view-switch="s">
      <KunEditorViewSwitch v-bind="s" />
    </template>
    <template #toolbar="api">
      <div class="flex flex-wrap items-center gap-0.5">
        <KunEditorToolbar v-bind="api" :items="toolbar" />
        <template v-if="api.adapters.uploadImage">
          <span class="bg-default-200 mx-1 h-5 w-px" aria-hidden="true" />
          <KunMarkdownImageDialog
            :api="api"
            :upload="api.adapters.uploadImage!"
          />
        </template>
      </div>
    </template>
  </KunEditor>
</template>
