// GET /patch/:id answers a work catalog deleted with this code, HTTP 410, and
// the deletion in data.
export const WORK_DELETED_CODE = 41001

export interface WorkDeletion {
  deleted_reason: string
  deleted_at: string
}

const WORK_DELETION_REASON_TEXT: Record<string, string> = {
  not_a_work: '该条目不是一部作品',
  out_of_scope: '不在收录范围内',
  spam_or_test: '垃圾或测试条目',
  other: '其他原因'
}

export const workDeletionReasonText = (reason: string) =>
  WORK_DELETION_REASON_TEXT[reason] ?? ''
