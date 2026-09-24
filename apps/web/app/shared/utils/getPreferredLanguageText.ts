const LANGUAGE_PRIORITY: Record<Language, Language[]> = {
  'en-us': ['en-us', 'ja-jp', 'zh-tw', 'zh-cn'],
  'ja-jp': ['ja-jp', 'en-us', 'zh-tw', 'zh-cn'],
  'zh-cn': ['zh-cn', 'zh-tw', 'ja-jp', 'en-us'],
  'zh-tw': ['zh-tw', 'zh-cn', 'ja-jp', 'en-us']
}

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

  for (const lang of LANGUAGE_PRIORITY[effectiveLocale]) {
    if (language[lang]) {
      return language[lang]
    }
  }

  return language.display_name || language.latin || ''
}

// pickPreferredLanguageRow runs the same priority chain over rows that carry
// their own language tag — an entity's introductions, where each language also
// carries which source wrote it.
export const pickPreferredLanguageRow = <T extends { lang: Language }>(
  rows: T[] | null | undefined,
  locale?: Language
): T | undefined => {
  if (!rows?.length) {
    return undefined
  }
  const effectiveLocale = locale ?? readTitleLanguagePreference()
  for (const lang of LANGUAGE_PRIORITY[effectiveLocale]) {
    const row = rows.find((r) => r.lang === lang)
    if (row) {
      return row
    }
  }
  return rows[0]
}

// getSecondaryLanguageText is the name the reader did NOT get: the second line
// under a character or a staff member, empty when it would only repeat the
// first. Latin sits last because it is a romanization, not a name anyone reads.
export const getSecondaryLanguageText = (
  language: KunLanguage | null | undefined,
  primary: string
): string => {
  if (!language) {
    return ''
  }
  const order: Language[] = ['ja-jp', 'zh-cn', 'zh-tw', 'en-us']
  return (
    order.map((lang) => language[lang]).find((v) => v && v !== primary) ?? ''
  )
}
