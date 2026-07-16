// Trust & Safety shapes. Report reason (GET /report/reasons) + the moderation
// inbox views (proxied by moyu's BFF at /admin/trust/*, mirroring the infra
// trust admin API). Integer enums are stable persisted codes — see
// app/constants/trust.ts for labels. Ambient global (no import), matching the
// app/shared/types/*.d.ts convention.

// A report reason from GET /report/reasons. severity: 1=low … 3=high.
interface ReportReason {
  key: string
  label: string
  severity: number
}

interface ReviewItemView {
  id: number
  site: string
  subject_kind: string
  subject_id: string
  source: number // 0 reports … 5 manual
  severity?: number
  classifier_score?: number
  report_weight_sum?: number
  priority: number
  status: number // 0 pending / 1 claimed / 2 actioned / 3 dismissed
  claimed_by?: number
  claimed_at?: string
  decided_by?: number
  decided_at?: string
  created_at: string
}

interface ReportView {
  id: number
  site: string
  subject_kind: string
  subject_id: string
  reporter_id: number
  reason_id: number
  note?: string
  subject_snapshot?: string
  subject_url?: string
  weight: number
  review_item_id?: number
  status: number
  created_at: string
}

interface ReviewItemDetail {
  item: ReviewItemView
  reports: ReportView[]
}

interface ReviewItemPage {
  items: ReviewItemView[]
  total: number
}
