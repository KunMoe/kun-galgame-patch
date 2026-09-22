import type { KunNsfwStance, KunSettingData } from '~/stores/settingStore'
import type { UserState } from '~/stores/userStore'

const STANCES: readonly string[] = ['hide', 'blur', 'show']

export const isNsfwStance = (v: unknown): v is KunNsfwStance =>
  typeof v === 'string' && STANCES.includes(v)

// The one fold of the account's two columns, per the OAuth content-preferences
// contract. `adult_confirmed` is NOT optional decoration: the upstream migration
// backfilled `nsfw_display = 'blur'` onto every existing account while leaving
// `adult_confirmed_at` null, so reading `nsfw_display` alone hands NSFW covers
// to nearly everyone who has never attested their age.
export const resolveAccountNsfwStance = (
  user: Pick<UserState, 'adult_confirmed' | 'nsfw_display'>
): KunNsfwStance =>
  user.adult_confirmed && isNsfwStance(user.nsfw_display)
    ? user.nsfw_display
    : 'hide'

// Signed in → the account decides, on every device. Signed out → this browser's
// cookie decides, which is the only store an anonymous reader has.
export const resolveNsfwStance = (
  setting: Pick<KunSettingData, 'kunNsfwEnable'>,
  user: Pick<UserState, 'id' | 'adult_confirmed' | 'nsfw_display'>
): KunNsfwStance => {
  if (user.id > 0) return resolveAccountNsfwStance(user)
  return isNsfwStance(setting.kunNsfwEnable) ? setting.kunNsfwEnable : 'hide'
}

// blur and show are the same request. Masking happens in the browser, so both
// count as the explicit opt-in the list gate asks for — a 'blur' reader who got
// content_limit=sfw would have nothing to mask.
export const stanceToContentLimit = (stance: KunNsfwStance): 'sfw' | 'all' =>
  stance === 'hide' ? 'sfw' : 'all'
