interface KunLanguage {
  'en-us': string
  'ja-jp': string
  'zh-cn': string
  'zh-tw': string
  // A work's name also carries catalog's display_name and latin, for a work
  // whose original language none of the four slots speaks.
  display_name?: string
  latin?: string
  machine_translated?: Language[]
}
