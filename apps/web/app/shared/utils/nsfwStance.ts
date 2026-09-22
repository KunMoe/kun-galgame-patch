import type { KunNsfwStance, KunSettingData } from '~/stores/settingStore'
import type { UserState } from '~/stores/userStore'

const STANCES: readonly string[] = ['hide', 'blur', 'show']

export const isNsfwStance = (v: unknown): v is KunNsfwStance =>
  typeof v === 'string' && STANCES.includes(v)

// The one fold of the account's two columns, and still exactly the OAuth
// content-preferences formula `adult_confirmed ? nsfw_display : 'hide'`. Age
// attestation retired on 2026-09-23, so upstream now sends `adult_confirmed`
// true for every account — the contract kept the formula rather than the gate,
// and so does this.
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
