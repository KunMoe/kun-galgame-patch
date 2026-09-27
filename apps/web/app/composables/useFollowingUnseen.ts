export const useFollowingUnseen = () => {
  const api = useApi()
  const unseen = useState('following-unseen', () => 0)

  const check = async () => {
    const res = await api.get<{ unseen_count: number }>(
      '/community/following/unseen'
    )
    if (res.code === 0) unseen.value = res.data.unseen_count
  }

  const markSeen = async (at: string | null) => {
    unseen.value = 0
    await api.put('/community/following/seen', at ? { at } : {})
  }

  return { unseen, check, markSeen }
}
