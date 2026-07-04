<script setup lang="ts">
// moyu's host-built image dialog for <KunEditor>, dropped into the #toolbar slot
// next to <KunEditorToolbar :items> (with the built-in 'image' button removed).
// It drives the editor purely through the slot API — `api.insertImage({ src })`
// for a ready URL — and runs moyu's own uploader (returns the domain-agnostic
// /image/<hash> token) for files/drag, so it can keep a persistent "recently
// used" history the editor knows nothing about (the headless split: the editor
// ships the insert primitive; the dialog / dropzone / history are the host's).
//
// KunPopover / KunInput / KunButton / KunIcon / KunTooltip are auto-imported by
// @kungal/ui-nuxt.
import { onMounted, ref } from 'vue'
import type { KunEditorToolbarApi } from '@kungal/editor-vue'

const props = defineProps<{
  api: KunEditorToolbarApi
  /** moyu's uploader (returns the stored URL) — `api.adapters.uploadImage`.
      Called directly so we can record the URL into history + insert it. */
  upload: (file: File) => Promise<string>
}>()

// Persisted across sessions so a user can re-insert images they used before.
// Just public /image/<hash> tokens — a convenience cache, not sensitive.
const HISTORY_KEY = 'moyu-editor-image-history'
const HISTORY_MAX = 12

const url = ref('')
const history = ref<string[]>([])
const busy = ref(false)
let popover: { close: () => void } | null = null
const setPopover = (el: unknown) => {
  popover = el as { close: () => void } | null
}
let fileEl: HTMLInputElement | null = null
const setFile = (el: unknown) => {
  fileEl = el as HTMLInputElement | null
}

onMounted(() => {
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    if (raw) history.value = JSON.parse(raw)
  } catch {
    /* corrupt / unavailable storage — start empty */
  }
})
const remember = (src: string) => {
  history.value = [src, ...history.value.filter((s) => s !== src)].slice(
    0,
    HISTORY_MAX
  )
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value))
  } catch {
    /* ignore quota / unavailable */
  }
}

const insertUrl = () => {
  const src = url.value.trim()
  if (!src) return
  props.api.insertImage({ src })
  remember(src)
  url.value = ''
  popover?.close()
}

const uploadFiles = async (files: File[]) => {
  // Accept image/* (and unknown-type — the <input accept> already restricts the
  // picker; some environments report an empty type).
  const imgs = files.filter((f) => !f.type || f.type.startsWith('image/'))
  if (!imgs.length) return
  busy.value = true
  try {
    for (const f of imgs) {
      const src = await props.upload(f)
      props.api.insertImage({ src })
      remember(src)
    }
  } finally {
    busy.value = false
  }
  popover?.close()
}

const pick = () => fileEl?.click()
const onFile = (e: Event) => {
  const input = e.target as HTMLInputElement
  // Copy to a static array BEFORE resetting value — `input.files` is live, so
  // clearing the input would empty it (breaks multi-select).
  const files = input.files ? Array.from(input.files) : []
  input.value = ''
  if (files.length) void uploadFiles(files)
}
const onDrop = (e: DragEvent) => {
  if (e.dataTransfer?.files?.length) {
    void uploadFiles(Array.from(e.dataTransfer.files))
  }
}
const insertFromHistory = (src: string) => {
  props.api.insertImage({ src })
  remember(src) // bump to front
  popover?.close()
}
</script>

<template>
  <KunPopover
    :ref="setPopover"
    position="bottom-start"
    :opaque="true"
    inner-class="w-80 space-y-3 p-3"
  >
    <template #trigger>
      <KunTooltip text="插入图片" :delay-show="300">
        <KunButton
          variant="light"
          size="sm"
          :is-icon-only="true"
          aria-label="插入图片"
        >
          <KunIcon name="lucide:image" />
        </KunButton>
      </KunTooltip>
    </template>

    <!-- 1. Insert by URL -->
    <form class="flex items-center gap-1" @submit.prevent="insertUrl">
      <KunInput
        v-model="url"
        type="url"
        size="sm"
        placeholder="粘贴图片链接…"
        class="flex-1"
      />
      <KunButton type="submit" variant="flat" color="primary" size="sm">
        插入
      </KunButton>
    </form>

    <!-- 2. Pick file(s) + drag-drop one or many -->
    <div
      class="border-default-300 text-default-500 hover:border-primary rounded-kun-md border border-dashed p-4 text-center text-sm transition-colors"
      @dragover.prevent
      @drop.prevent="onDrop"
    >
      <KunIcon name="lucide:cloud-upload" class="mx-auto mb-1 text-xl" />
      <p>拖拽一张或多张图片到此处</p>
      <KunButton
        variant="flat"
        size="sm"
        class="mt-2"
        :loading="busy"
        @click="pick"
      >
        选择图片
      </KunButton>
      <input
        :ref="setFile"
        type="file"
        accept="image/*"
        multiple
        hidden
        @change="onFile"
      />
    </div>

    <!-- 3. Re-insert from the (host-kept, persisted) history -->
    <div v-if="history.length">
      <div class="text-default-500 mb-1 text-xs">最近使用</div>
      <div class="grid grid-cols-4 gap-1">
        <button
          v-for="(src, i) in history"
          :key="i"
          type="button"
          class="hover:ring-primary overflow-hidden rounded ring-2 ring-transparent"
          @click="insertFromHistory(src)"
        >
          <img :src="src" alt="" class="h-14 w-full object-cover" />
        </button>
      </div>
    </div>
  </KunPopover>
</template>
