// Session-cached report-reason catalog (GET /report/reasons). Fetched once and
// shared across every <ReportButton> / the modal so N mounts don't each hit the
// endpoint. Reasons are served live by the BFF from the trust registry (with a
// seeded fallback), so this list stays in sync with infra. useState (not module
// refs) keeps it SSR-safe, matching useReportModal / useAuthModal.
export const useReportReasons = () => {
  const reasons = useState<ReportReason[]>('kun-report-reasons', () => [])
  const loaded = useState('kun-report-reasons-loaded', () => false)

  const load = async () => {
    if (loaded.value) return
    const api = useApi()
    const res = await api.get<ReportReason[]>('/report/reasons')
    if (res.code === 0 && res.data) {
      reasons.value = res.data
      loaded.value = true
    }
  }
  return { reasons, load }
}
