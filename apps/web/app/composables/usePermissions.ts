// Only decides what to show; every write is checked again by the API.
export const usePermissions = () => {
  const mine = useState<string[]>('moyu-permissions', () => [])
  const api = useApi()
  const userStore = useUserStore()

  const refresh = async () => {
    if (!userStore.user.id) {
      mine.value = []
      return
    }
    const res = await api.get<{ permissions: string[] }>('/auth/me/permissions')
    mine.value = res.code === 0 ? res.data.permissions : []
  }

  const can = (key: string) => mine.value.includes(key)
  const canAny = (keys: readonly string[]) => keys.some(can)

  return { can, canAny, refresh }
}
