import type { KunThemePreference } from '~/constants/top-bar'
import type { KunNsfwPreference } from '~/stores/settingStore'

// The two v-models behind every 主题 / 内容显示 switch (top bar, mobile menu,
// 系统设置). Flipping the content limit reloads: the gate is baked into the
// SSR payload and into every list already on screen, so a surface that only
// wrote the store would leave the page showing the previous gate's rows.
export const useKunDisplayPreference = () => {
  const colorMode = useColorMode()
  const settingStore = useSettingStore()

  const theme = computed<KunThemePreference>({
    get: () => (colorMode.preference as KunThemePreference) ?? 'system',
    set: (v) => {
      colorMode.preference = v
    }
  })

  const contentLimit = computed<KunNsfwPreference>({
    get: () => settingStore.data.kunNsfwEnable,
    set: (v) => {
      settingStore.setNsfwPreference(v)
      // Persist synchronously before the reload: the plugin's own write rides a
      // pre-flush watcher (a microtask), and location.reload() would race it.
      settingStore.$persist()
      if (import.meta.client) location.reload()
    }
  })

  return { theme, contentLimit }
}
