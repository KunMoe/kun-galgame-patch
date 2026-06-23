// readTitleLanguagePreference returns the user's "游戏标题优先语言" setting
// (settingStore.titleLanguage). The store is cookie-backed, so this resolves
// correctly during SSR too. Falls back to 'ja-jp' (the default) when Pinia isn't
// active (non-component context, e.g. a unit test) or the key is unset on an old
// cookie.
const readTitleLanguagePreference = (): Language => {
  try {
    return useSettingStore().data.titleLanguage ?? 'ja-jp'
  } catch {
    return 'ja-jp'
  }
}

export const getPreferredLanguageText = (
  language: KunLanguage | null | undefined,
  locale?: Language
): string => {
  if (!language) {
    return ''
  }

  // When the caller doesn't pin a locale, honor the user's title-language
  // preference so game names follow it on EVERY page — not just the Galgame
  // card, which used to be the only caller passing it explicitly. Reading the
  // store here (rather than at each call site) keeps all ~15 usages in sync and
  // reactive: a computed/template that calls this re-renders when the setting
  // changes. Falls back to 'zh-cn'.
  const effectiveLocale = locale ?? readTitleLanguagePreference()

  const languagePriority: Record<Language, Language[]> = {
    'en-us': ['en-us', 'ja-jp', 'zh-tw', 'zh-cn'],
    'ja-jp': ['ja-jp', 'en-us', 'zh-tw', 'zh-cn'],
    'zh-cn': ['zh-cn', 'zh-tw', 'ja-jp', 'en-us'],
    'zh-tw': ['zh-tw', 'zh-cn', 'ja-jp', 'en-us']
  }

  const priorities = languagePriority[effectiveLocale]

  for (const lang of priorities) {
    if (language[lang]) {
      return language[lang]
    }
  }

  return ''
}
