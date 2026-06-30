// Client-only "is a Galgame releasing today?" check, used to nudge the calendar
// nav entry (turns primary + "有新作发售"). Reuses the current-month calendar
// payload (it already carries `today`), keyed once per app load so it's fetched
// at most once. release fields live on the nested `galgame` object.
export const useGalgameReleaseToday = () => {
  const api = useApi()

  const { data } = useAsyncData<CalendarMonthResponse | null>(
    'galgame-release-today',
    async () => {
      const res = await api.get<CalendarMonthResponse>('/galgame/calendar')
      return res.code === 0 ? res.data : null
    },
    { server: false }
  )

  const hasReleaseToday = computed(
    () =>
      !!data.value?.items.some(
        (g) =>
          g.galgame?.release_precision === 'day' &&
          g.galgame?.release_date?.slice(0, 10) === data.value!.today
      )
  )

  return { hasReleaseToday }
}
