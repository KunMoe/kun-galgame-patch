import { ref } from 'vue'
import type { Ref } from 'vue'

export interface KunFilePickerOptions {
  accept?: string
  multiple?: boolean
  // Per-file size limit in bytes. A file exceeding this aborts the
  // entire selection (consistent with native form validation, which
  // rejects the whole input rather than silently dropping items).
  maxSize?: number
  onError?: (message: string, file: File) => void
}

export interface KunFilePickerReturn {
  files: Ref<File[]>
  pickFiles: () => void
  clear: () => void
}

// Programmatic file picker — creates a transient hidden <input
// type="file">, opens the native browser dialog, then discards the
// input. Decouples file selection from any visual trigger so consumers
// can wire it to any button / card / link.
export const useFilePicker = (
  options: KunFilePickerOptions = {}
): KunFilePickerReturn => {
  const files = ref<File[]>([])

  const pickFiles = () => {
    if (typeof document === 'undefined') {
      // SSR guard — pickFiles is only meaningful on the client.
      return
    }
    const input = document.createElement('input')
    input.type = 'file'
    if (options.accept) {
      input.accept = options.accept
    }
    if (options.multiple) {
      input.multiple = true
    }
    input.addEventListener('change', (event) => {
      const selected = Array.from(
        (event.target as HTMLInputElement).files ?? []
      )
      if (options.maxSize !== undefined) {
        const tooBig = selected.find((f) => f.size > options.maxSize!)
        if (tooBig) {
          options.onError?.(
            `${tooBig.name} 超过大小限制 (${formatBytes(options.maxSize)})`,
            tooBig
          )
          return
        }
      }
      files.value = selected
    })
    input.click()
  }

  const clear = () => {
    files.value = []
  }

  return { files, pickFiles, clear }
}

const formatBytes = (bytes: number) => {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}
