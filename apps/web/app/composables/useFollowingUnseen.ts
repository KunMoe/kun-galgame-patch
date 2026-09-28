export const useFollowingUnseen = () => {
  const api = useApi()
  const unseen = useState('following-unseen', () => 0)
  const generation = useState('following-unseen-generation', () => 0)

  // A count request still in flight when /following marks the feed seen lands
  // after it and paints the stale count back.
  const check = async () => {
    const asked = generation.value
    const res = await api.get<{ unseen_count: number }>(
      '/community/following/unseen'
    )
    if (res.code === 0 && asked === generation.value) {
      unseen.value = res.data.unseen_count
    }
  }

  const markSeen = async (at: string | null) => {
    generation.value++
    unseen.value = 0
    await api.put('/community/following/seen', at ? { at } : {})
  }

  return { unseen, check, markSeen }
}
