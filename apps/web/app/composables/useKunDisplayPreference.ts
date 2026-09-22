import type { KunThemePreference } from '~/constants/top-bar'
import type { KunNsfwStance } from '~/stores/settingStore'

// The two v-models behind every 主题 / 内容显示 switch (top bar, mobile menu,
// 系统设置). The content one is a three-state stance; useKunNsfwStance owns what
// writing it means (account write when signed in, cookie when not) and when the
// page has to be rebuilt for it.
export const useKunDisplayPreference = () => {
  const colorMode = useColorMode()
  const { stance, setStance } = useKunNsfwStance()

  const theme = computed<KunThemePreference>({
    get: () => (colorMode.preference as KunThemePreference) ?? 'system',
    set: (v) => {
      colorMode.preference = v
    }
  })

  const contentStance = computed<KunNsfwStance>({
    get: () => stance.value,
    set: (v) => {
      setStance(v)
    }
  })

  return { theme, contentStance }
}
