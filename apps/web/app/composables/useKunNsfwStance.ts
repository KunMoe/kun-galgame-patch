import type { KunNsfwStance } from '~/stores/settingStore'

interface NsfwDisplayReply {
  nsfw_display: KunNsfwStance
  adult_confirmed_at: string | null
}

// The single owner of "what stance is in force, and how does a switch change
// it". Three surfaces render the switch (top bar, mobile menu, 系统设置) and
// each one used to carry its own write.
export const useKunNsfwStance = () => {
  const settingStore = useSettingStore()
  const userStore = useUserStore()
  const api = useApi()

  // useState, not a module ref: the switches live in three separate component
  // trees and a switch in one must disable the others while the write is out.
  const pending = useState('kun-nsfw-stance-pending', () => false)

  const stance = computed<KunNsfwStance>(() =>
    resolveNsfwStance(settingStore.data, userStore.user)
  )

  const setStance = async (next: KunNsfwStance) => {
    if (pending.value || next === stance.value) return

    // hide ⇄ blur|show changes what the API is asked for; blur ⇄ show does not,
    // so only the first needs the page rebuilt. Reloading on the second would
    // throw away a scroll position to repaint the same rows.
    const reloadNeeded =
      stanceToContentLimit(next) !== stanceToContentLimit(stance.value)

    if (userStore.user.id <= 0) {
      settingStore.setNsfwStance(next)
      if (reloadNeeded && import.meta.client) location.reload()
      return
    }

    const previous = userStore.user.nsfw_display
    pending.value = true
    userStore.setNsfwStance(next)
    const res = await api.put<NsfwDisplayReply>('/auth/me/nsfw', {
      nsfw_display: next
    })
    pending.value = false

    if (res.code !== 0) {
      userStore.setNsfwStance(previous || 'hide')
      // 40399 already raises its own long-lived "log out and back in" toast
      // from useApi, so saying anything more here would stack two messages on
      // one refusal.
      if (res.code !== 40399) {
        useKunMessage(res.message || '设置失败，请稍后再试', 'error')
      }
      return
    }

    userStore.setNsfwStance(
      res.data.nsfw_display,
      !!res.data.adult_confirmed_at
    )
    if (reloadNeeded && import.meta.client) location.reload()
  }

  return { stance, setStance, pending }
}
