// One fetch for every unread badge: the top-bar bell and the /message center
// tabs both read the message store this fills. Called on layout mount and again
// after a page marks its category read, so the counts never drift from the
// list the user just cleared.
export const useUnreadCounts = () => {
  const api = useApi()
  const messageStore = useMessageStore()

  const refresh = async () => {
    const res =
      await api.get<Record<string, number>>('/message/unread/counts')
    if (res.code === 0) messageStore.setCounts(res.data ?? {})
  }

  return { refresh }
}
