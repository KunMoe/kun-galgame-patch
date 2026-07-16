// Shared state for the ONE global report modal (mounted at app.vue root). A
// <ReportButton> only calls open(); the modal renders at the stable root so a
// triggering ⋯ popover closing doesn't teleport-tear it down mid-animation.
// useState (not a module ref) keeps it SSR-safe, matching useAuthModal.
export interface ReportTarget {
  subjectKind: string
  subjectId: string | number
  snapshot?: string
  // Absolute deep-link to the reported content, so the moderator console opens
  // it in context (must be absolute — clicked inside infra's console).
  subjectUrl?: string
}

export const useReportModal = () => {
  const isOpen = useState('kun-report-modal-open', () => false)
  const target = useState<ReportTarget | null>('kun-report-modal-target', () => null)

  const open = (t: ReportTarget) => {
    target.value = t
    isOpen.value = true
  }

  return { isOpen, target, open }
}
