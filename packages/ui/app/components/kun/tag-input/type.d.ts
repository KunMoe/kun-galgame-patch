import type { KunUIColor, KunUISize } from '../ui/type'

export type KunTagInputVariant = 'bordered' | 'flat'

export type KunTagInputInvalidReason =
  | 'duplicate'
  | 'too-long'
  | 'too-short'
  | 'max-reached'
  | 'custom'

export type KunTagInputValidator = (
  tag: string,
  all: string[]
) => true | string

export interface KunTagInputProps {
  label?: string
  placeholder?: string
  helperText?: string
  error?: string

  // limits
  maxTags?: number
  maxTagLength?: number
  minTagLength?: number

  // dedupe / normalize
  allowDuplicates?: boolean
  caseSensitive?: boolean
  trim?: boolean
  transform?: (raw: string) => string
  validate?: KunTagInputValidator

  // delimiters
  splitChars?: (string | RegExp)[]
  splitOnPaste?: boolean

  // behavior
  confirmOnBlur?: boolean
  respectComposition?: boolean

  // style
  color?: KunUIColor
  size?: KunUISize
  variant?: KunTagInputVariant
  disabled?: boolean
  readonly?: boolean
  showCounter?: boolean
  className?: string
}
