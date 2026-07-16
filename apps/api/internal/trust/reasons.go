// Package trust is moyu's BFF integration with the infra Trust & Safety
// service: report intake (Phase 1), enforcement callbacks (Phase 2), and the
// moderator inbox proxy (Phase 3).
package trust

// ReportReason mirrors an infra-seeded global report reason
// (trust_report_reason). The reasons are fetched live from the trust S2S feed
// (GET /api/v1/trust/report-reasons — TrustService.Reasons); this constant is
// only the FALLBACK used when trust is unconfigured/unreachable, so the report
// dropdown always has options. Keep in sync with infra SeedReasons.
type ReportReason struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Severity int    `json:"severity"`
}

// GlobalReasons are the six seeded global reasons (severity 1=low … 3=high),
// used as the offline fallback for Reasons().
var GlobalReasons = []ReportReason{
	{Key: "abuse", Label: "辱骂骚扰", Severity: 2},
	{Key: "spam", Label: "垃圾信息", Severity: 1},
	{Key: "illegal", Label: "违法内容", Severity: 3},
	{Key: "rating_mislabel", Label: "分级标注错误", Severity: 1},
	{Key: "copyright", Label: "版权侵权", Severity: 2},
	{Key: "other", Label: "其他", Severity: 1},
}
