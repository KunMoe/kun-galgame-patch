<script setup lang="ts">
// Milkdown core
import { Editor, rootCtx, defaultValueCtx } from '@milkdown/kit/core'
import { Milkdown, useEditor } from '@milkdown/vue'
import { OverlayScrollbarsComponent } from 'overlayscrollbars-vue'
import type { PartialOptions } from 'overlayscrollbars'
import { commonmark } from '@milkdown/kit/preset/commonmark'
import { gfm } from '@milkdown/kit/preset/gfm'
// Milkdown Plugins
import { history } from '@milkdown/kit/plugin/history'
import { listener, listenerCtx } from '@milkdown/kit/plugin/listener'
import { clipboard } from '@milkdown/kit/plugin/clipboard'
import { indent } from '@milkdown/kit/plugin/indent'
import { trailing } from '@milkdown/kit/plugin/trailing'
import { usePluginViewFactory } from '@prosemirror-adapter/vue'
import { upload, uploadConfig } from '@milkdown/kit/plugin/upload'

// Custom plugins
import { activeTab } from './atom'
import { createKunUploader, kunUploadWidgetFactory } from './plugins/upload/uploader'
import { tooltipFactory } from '@milkdown/kit/plugin/tooltip'
import Tooltip from './plugins/tooltip/Tooltip.vue'
import { slashFactory } from '@milkdown/kit/plugin/slash'
import Mention from './plugins/mention/Mention.vue'
import { setMentionApiBase } from './plugins/mention/config'
import { replaceAll } from '@milkdown/kit/utils'
import {
  stopLinkCommand,
  linkCustomKeymap
} from './plugins/stop-link/stopLinkPlugin'
import { kunSpoilerPlugin } from './plugins/spoiler/spoilerPlugin'

// Code Block
import { defaultKeymap, indentWithTab } from '@codemirror/commands'
import { keymap, EditorView } from '@codemirror/view'
import {
  codeBlockComponent,
  codeBlockConfig,
  type CodeBlockConfig
} from '@milkdown/kit/component/code-block'
import { basicSetup } from 'codemirror'
import {
  chevronDownIcon,
  clearIcon,
  copyIcon,
  editIcon,
  searchIcon,
  visibilityOffIcon
} from './plugins/code/icons'
import { languages } from '@codemirror/language-data'
import { kunCM } from './codemirror/theme'

// katex
import { blockKatexSchema } from './plugins/katex/blockKatex'
import { mathInlineSchema } from './plugins/katex/inlineKatex'
import { toggleLatexCommand } from './plugins/katex/command'
import {
  mathBlockInputRule,
  mathInlineInputRule
} from './plugins/katex/inputRule'
import { remarkMathBlockPlugin, remarkMathPlugin } from './plugins/katex/remark'
import katex from 'katex'
import type { KatexOptions } from 'katex'

const props = withDefaults(
  defineProps<{
    valueMarkdown: string
    language: Language
    // When false, every image affordance is removed: the toolbar upload-image
    // button, the sticker picker (stickers are images), AND the upload plugin
    // (paste / drag-drop image upload). Used by the galgame intro editor — intro
    // carries no images; they move to the Wiki gallery. Defaults true so
    // comments / resource notes are unaffected.
    allowImage?: boolean
  }>(),
  { allowImage: true }
)

const emits = defineEmits<{
  saveMarkdown: [markdown: string]
}>()

const valueMarkdown = computed(() => props.valueMarkdown)

// overlayscrollbars for the rich-text editing surface: native scroll kept, the
// scrollbar replaced by a thin overlay handle that auto-hides on pointer leave
// (themed in editor.css via --os-*). Replaces the glaring native dark-mode
// scrollbar that .milkdown used to render. autoHide:'leave' keeps it out of the
// way while typing.
const osOptions: PartialOptions = {
  scrollbars: {
    autoHide: 'leave',
    autoHideDelay: 500
  }
}

const tooltip = tooltipFactory('Text')
const mention = slashFactory('kun-mention')
const pluginViewFactory = usePluginViewFactory()

// Captured in setup (Nuxt context) so the runtime API base can be handed to
// the upload plugin, whose uploader runs outside setup (paste/drop handlers).
const runtimeConfig = useRuntimeConfig()
const apiBase =
  (runtimeConfig.public.apiBase as string) || 'http://127.0.0.1:5214/api/v1'
// Hand the API base to the mention dropdown (rendered outside Nuxt's setup tree).
setMentionApiBase(apiBase)
const container = ref<HTMLElement | null>(null)
const toolbar = ref<HTMLElement | null>(null)
const editorContent = ref('')

const renderLatex = (content: string, options?: KatexOptions) => {
  const html = katex.renderToString(content, {
    ...options,
    throwOnError: false,
    displayMode: true
  })
  return html
}

const editorInfo = useEditor((root) => {
  const editor = Editor.make()
    .config((ctx) => {
      ctx.set(rootCtx, root)
      ctx.set(defaultValueCtx, valueMarkdown.value)

      const listener = ctx.get(listenerCtx)
      listener.markdownUpdated((ctx, markdown, prevMarkdown) => {
        if (markdown !== prevMarkdown) {
          editorContent.value = markdown
          emits('saveMarkdown', markdown)
        }
      })

      // Only configure the upload plugin when it's actually registered below;
      // touching uploadConfig.key without `.use(upload)` throws (missing slice).
      if (props.allowImage) {
        ctx.update(uploadConfig.key, (prev) => ({
          ...prev,
          uploader: createKunUploader(apiBase),
          uploadWidgetFactory: kunUploadWidgetFactory
        }))
      }

      ctx.set(tooltip.key, {
        view: pluginViewFactory({
          component: Tooltip
        })
      })

      ctx.set(mention.key, {
        view: pluginViewFactory({
          component: Mention
        })
      })

      const extensions = [
        kunCM(),
        EditorView.lineWrapping,
        keymap.of(defaultKeymap.concat(indentWithTab)),
        basicSetup
      ]
      // if (theme) {
      //   extensions.push(theme)
      // }

      ctx.update(codeBlockConfig.key, (defaultConfig) => ({
        extensions,
        languages,
        expandIcon: chevronDownIcon,
        searchIcon: searchIcon,
        clearSearchIcon: clearIcon,
        searchPlaceholder: '搜索咒文',
        copyText: '复制咒文',
        copyIcon: copyIcon,
        onCopy: () => {},
        noResultText: '无结果',
        renderLanguage: defaultConfig.renderLanguage,
        previewLoading: '加载中...',
        renderPreview: defaultConfig.renderPreview,
        previewToggleButton: (previewOnlyMode) => {
          const icon = previewOnlyMode ? editIcon : visibilityOffIcon
          const text = previewOnlyMode ? '编辑' : '隐藏'
          return [icon, text].map((v) => v.trim()).join(' ')
        },
        previewLabel: defaultConfig.previewLabel
        // previewLoading: config.previewLoading || defaultConfig.previewLoading,
        // previewOnlyByDefault:
        //   config.previewOnlyByDefault ?? defaultConfig.previewOnlyByDefault
      }))

      const katexOptions: KatexOptions = {}

      ctx.update(codeBlockConfig.key, (prev) => ({
        ...prev,
        renderPreview: (language, content, applyPreview) => {
          if (language.toLowerCase() === 'latex' && content.length > 0) {
            return renderLatex(content, katexOptions)
          }
          const renderPreview = prev.renderPreview
          return renderPreview(language, content, applyPreview)
        }
      }))
    })
    .use(history)
    .use(commonmark)
    .use(gfm)
    .use(listener)
    .use(clipboard)
    .use(indent)
    .use(trailing)
    .use(tooltip)
    .use(mention)
    .use(codeBlockComponent)
    .use([kunSpoilerPlugin, stopLinkCommand, linkCustomKeymap].flat())
    .use(remarkMathPlugin)
    .use(remarkMathBlockPlugin)
    .use(mathInlineSchema)
    .use(mathInlineInputRule)
    .use(mathBlockInputRule)
    .use(blockKatexSchema)
    .use(toggleLatexCommand)

  // Image upload (paste / drag-drop) is opt-out: the intro editor skips it so
  // no image can be added there. The uploadConfig above is guarded the same way.
  if (props.allowImage) {
    editor.use(upload)
  }
  return editor
})

const textCount = computed(() => markdownToText(props.valueMarkdown).length)

watch(
  () => [props.language],
  () => {
    editorInfo.get()?.action(replaceAll(valueMarkdown.value))
  }
)
</script>

<template>
  <div ref="container" class="space-y-3">
    <KunMilkdownPluginsMenu
      ref="toolbar"
      :editor-info="editorInfo"
      :allow-image="allowImage"
    />

    <template v-if="activeTab === 'preview'">
      <!-- overlayscrollbars wraps <Milkdown /> as an ANCESTOR (.os-host >
           .os-viewport > .os-content > .milkdown), so ProseMirror's own DOM is
           left untouched. The 500px cap + scrolling moved here off .milkdown. -->
      <OverlayScrollbarsComponent
        :options="osOptions"
        class="kun-editor-scroll max-h-[500px]"
        defer
      >
        <Milkdown />
      </OverlayScrollbarsComponent>

      <div class="flex items-center justify-between text-sm">
        <slot name="footer" />

        <div class="flex shrink-0 items-center gap-2">
          <KunChip color="success">
            <KunIcon
              name="fa6-brands:markdown"
              class="text-success-700 dark:text-success"
            />
            Markdown 支持
          </KunChip>
          <span>
            {{ `${textCount} 字` }}
          </span>
        </div>
      </div>

      <div class="text-default-500 text-sm">
        <template v-if="allowImage">
          特殊语法: 您可以使用 ||隐藏文本|| 来隐藏图片或者文字 (目前依然禁止 R18
          图片内容)
        </template>
        <template v-else>
          特殊语法: 您可以使用 ||隐藏文本|| 来隐藏文字。简介不支持图片，图片请使用
          Wiki 画廊
        </template>
      </div>
    </template>
  </div>
</template>
