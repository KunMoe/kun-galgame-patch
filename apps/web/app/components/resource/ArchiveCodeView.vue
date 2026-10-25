<script setup lang="ts">
import type { Compartment, Extension } from '@codemirror/state'
import type { EditorView } from '@codemirror/view'

const props = defineProps<{ text: string; fileName: string; wrap: boolean }>()

const SEARCH_PHRASES = {
  Find: '查找',
  next: '下一个',
  previous: '上一个',
  all: '全部',
  'match case': '区分大小写',
  regexp: '正则',
  'by word': '全词匹配',
  close: '关闭',
  'current match': '当前匹配',
  'on line': '行',
  'Go to line': '跳转到行',
  go: '跳转'
}

const host = ref<HTMLElement | null>(null)
const ready = ref(false)

let view: EditorView | null = null
let wrapping: Compartment | null = null
let lineWrapping: Extension = []
let disposed = false

const loadEditor = async () => {
  const [viewMod, stateMod, languageMod, dataMod, searchMod, presetMod] =
    await Promise.all([
      import('@codemirror/view'),
      import('@codemirror/state'),
      import('@codemirror/language'),
      import('@codemirror/language-data'),
      import('@codemirror/search'),
      import('@kungal/editor-core/preset')
    ])
  const { EditorView, lineNumbers, highlightSpecialChars, keymap } = viewMod
  const description = languageMod.LanguageDescription.matchFilename(
    dataMod.languages,
    props.fileName
  )
  const language = description
    ? await description.load().catch(() => null)
    : null
  if (disposed || !host.value) return

  lineWrapping = EditorView.lineWrapping
  wrapping = new stateMod.Compartment()
  view = new EditorView({
    parent: host.value,
    state: stateMod.EditorState.create({
      doc: props.text,
      extensions: [
        EditorView.theme({
          '&': {
            height: '100%',
            minHeight: '0',
            fontSize: '13px',
            borderRadius: '0',
            backgroundColor: 'transparent'
          },
          '.cm-scroller': {
            fontFamily:
              'ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace',
            scrollbarWidth: 'thin'
          },
          '.cm-content': { padding: '0.5rem 0' },
          '.cm-line': { fontSize: '13px', padding: '0.05rem 0.75rem' },
          '.cm-line:hover': { backgroundColor: 'transparent' },
          '.cm-gutters': {
            backgroundColor: 'transparent',
            border: 'none',
            color: 'var(--color-default-400)'
          },
          '.cm-gutterElement': { fontSize: '12px' },
          '.cm-panels': {
            backgroundColor: 'var(--color-content2)',
            color: 'inherit'
          },
          '.cm-panels-top': {
            borderBottom: '1px solid var(--color-default-200)'
          },
          '.cm-search': {
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            gap: '0.375rem 0.5rem',
            padding: '0.5rem 2rem 0.5rem 0.75rem',
            fontSize: '12px'
          },
          '.cm-search label': {
            display: 'inline-flex',
            alignItems: 'center',
            gap: '0.25rem',
            fontSize: '12px'
          },
          '.cm-textfield, .cm-button': {
            margin: '0',
            fontSize: '12px',
            color: 'inherit',
            borderRadius: 'var(--radius-kun-md)'
          },
          '.cm-textfield': {
            padding: '0.25rem 0.625rem',
            border: '1px solid var(--color-default-300)',
            backgroundColor: 'transparent'
          },
          '.cm-button': {
            padding: '0.25rem 0.625rem',
            border: 'none',
            backgroundImage: 'none',
            backgroundColor: 'var(--color-default-100)'
          },
          '.cm-button:active': {
            backgroundImage: 'none',
            backgroundColor: 'var(--color-default-200)'
          },
          '.cm-searchMatch': {
            backgroundColor:
              'color-mix(in oklch, var(--color-warning) 30%, transparent)',
            borderRadius: '2px'
          },
          '.cm-searchMatch.cm-searchMatch-selected': {
            backgroundColor:
              'color-mix(in oklch, var(--color-primary) 45%, transparent)'
          }
        }),
        presetMod.kunCM(),
        lineNumbers(),
        highlightSpecialChars(),
        stateMod.EditorState.readOnly.of(true),
        stateMod.EditorState.phrases.of(SEARCH_PHRASES),
        searchMod.search({ top: true }),
        keymap.of(searchMod.searchKeymap),
        wrapping.of(props.wrap ? lineWrapping : []),
        language ?? []
      ]
    })
  })
  ready.value = true
}

onMounted(loadEditor)

onBeforeUnmount(() => {
  disposed = true
  view?.destroy()
})

watch(
  () => props.text,
  (text) =>
    view?.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: text }
    })
)

watch(
  () => props.wrap,
  (wrap) =>
    wrapping &&
    view?.dispatch({ effects: wrapping.reconfigure(wrap ? lineWrapping : []) })
)
</script>

<template>
  <div class="relative h-full min-h-0">
    <div ref="host" class="h-full" />
    <div
      v-if="!ready"
      class="text-default-400 absolute inset-0 flex items-center justify-center gap-2 text-sm"
    >
      <KunIcon name="lucide:loader-circle" class="size-4 animate-spin" />
      正在准备查看器
    </div>
  </div>
</template>
