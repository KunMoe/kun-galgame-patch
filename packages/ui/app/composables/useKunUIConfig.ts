import { inject, provide, type InjectionKey } from 'vue'
import type { KunUIRounded } from '../components/kun/ui/type'

// Global KunUI defaults applied to every component in a Vue subtree
// (or whole app if provided at root). Each component still accepts
// per-instance overrides via its own props — the precedence chain is:
//
//   per-instance prop  >  useKunUIConfig provider  >  built-in default
//
// Set up once near the app root:
//   import { provideKunUIConfig } from '@kun/ui/composables/useKunUIConfig'
//   provideKunUIConfig({ rounded: 'lg' })
export interface KunUIConfig {
  /** Default border radius bucket. Affects all components that
   *  don't pass a `rounded` prop AND aren't shape-locked (Avatar /
   *  Chip / dot Badge / Switch thumb stay circular regardless). */
  rounded: KunUIRounded
}

const KUN_UI_CONFIG_KEY: InjectionKey<KunUIConfig> = Symbol('kun-ui-config')

// Built-in defaults — match the look of v0.0.1 → v0.2.x so existing
// apps don't shift visually unless they opt in via provideKunUIConfig.
export const KUN_UI_DEFAULT_CONFIG: KunUIConfig = {
  rounded: 'md',
}

export const provideKunUIConfig = (config: Partial<KunUIConfig>) => {
  provide(KUN_UI_CONFIG_KEY, { ...KUN_UI_DEFAULT_CONFIG, ...config })
}

export const useKunUIConfig = (): KunUIConfig => {
  return inject(KUN_UI_CONFIG_KEY, KUN_UI_DEFAULT_CONFIG)
}
