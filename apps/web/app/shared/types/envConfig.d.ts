// Matches apps/api/internal/envconfig (GET /admin/setting/env).
type EnvVariableKind = 'bootstrap' | 'topology' | 'secret' | 'plain'
type EnvValueSource = 'env' | 'store' | 'default'
type EnvVariableIssue =
  | 'panel_redundant'
  | 'panel_overrides_store'
  | 'restart_pending'
  | 'not_read'

interface EnvValue {
  is_set: boolean
  plain: string | null
  fingerprint: string | null
  hint: string | null
}

interface EnvVariable {
  variable: string
  variable_kind: EnvVariableKind
  resolved_from: EnvValueSource | null
  running: EnvValue | null
  default_value: string | null
  stored: EnvValue | null
  stored_at: string | null
  issues: EnvVariableIssue[]
  is_storable: boolean
}

interface EnvConfig {
  is_store_enabled: boolean
  loaded_at: string
  variables: EnvVariable[]
  unread_variables: string[]
}

interface EnvChange {
  id: number
  variable: string
  change_kind: 'set' | 'delete' | 'import'
  shown_before: string
  shown_after: string
  change_note: string
  changed_by: KunUser | null
  created_at: string
}

interface AdminDlsiteSetting {
  link_template: string
  coupon_url: string
}
