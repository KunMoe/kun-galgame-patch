import type { KunNsfwStance } from '~/stores/settingStore'

interface NsfwDisplayReply {
  nsfw_display: KunNsfwStance
  adult_confirmed_at: string | null
}

// OAuth refuses blur / show on an account that never attested its age. The
// attestation itself belongs to the account centre and to nowhere else, so the
// only thing this site may do with an 18008 is point at it.
const ATTESTATION_REQUIRED = 18008

// The single owner of "what stance is in force, and how does a switch change
// it". Three surfaces render the switch (top bar, mobile menu, 系统设置) and
// each one used to carry its own write.
export const useKunNsfwStance = () => {
  const settingStore = useSettingStore()
  const userStore = useUserStore()
  const api = useApi()

  // useState, not a module ref: the modal it drives is mounted once in the
  // layout while the switches live in three separate component trees.
  const attestationOpen = useState('kun-nsfw-attestation', () => false)
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
      if (res.code === ATTESTATION_REQUIRED) attestationOpen.value = true
      // 40399 already raises its own long-lived "log out and back in" toast
      // from useApi, so saying anything more here would stack two messages on
      // one refusal.
      else if (res.code !== 40399) {
        useKunMessage(res.message || '设置失败，请稍后再试', 'error')
      }
      return
    }

    // The reply is the account's truth after the write, including an
    // attestation the reader completed at the account centre since this tab
    // loaded — which is exactly the round trip the 18008 dialog sends them on.
    userStore.setNsfwStance(
      res.data.nsfw_display,
      !!res.data.adult_confirmed_at
    )
    if (reloadNeeded && import.meta.client) location.reload()
  }

  return { stance, setStance, pending, attestationOpen }
}
