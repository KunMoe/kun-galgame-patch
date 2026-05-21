import type { KunUIColor, KunUISize, KunUIVariant } from '../ui/type'

export interface KunFileInputProps {
  // Native input attribute pass-through.
  accept?: string
  // Multi-file mode toggles which v-model is active:
  //   `v-model` (default) → File | null         (when `multiple: false`)
  //   `v-model:files`     → File[]              (when `multiple: true`)
  // Don't mix the two; the unused model stays at its default value.
  multiple?: boolean
  // Per-file size cap in bytes. Exceeding aborts the whole selection
  // (matches native form validation and useFilePicker semantics).
  maxSize?: number
  // Helper text shown beneath the trigger when there's no error.
  hint?: string
  // Error text shown beneath the trigger; takes precedence over hint.
  error?: string
  disabled?: boolean
  // Built-in trigger button appearance. Ignored if default slot
  // provides a custom trigger.
  triggerText?: string
  triggerIcon?: string
  triggerVariant?: KunUIVariant
  triggerColor?: KunUIColor
  triggerSize?: KunUISize
  fullWidth?: boolean
  // Render the picked file name (single) or count (multiple) next to
  // the trigger. Default true.
  showFileName?: boolean
  className?: string
}
