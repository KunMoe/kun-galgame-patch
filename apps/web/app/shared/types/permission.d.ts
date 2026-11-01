// Matches apps/api/internal/permission and apps/api/pkg/perm.
interface PermissionGroup {
  key: string
  label: string
}

interface PermissionEntry {
  key: string
  group: string
  label: string
  description: string
  sensitive: boolean
}

interface PermissionOverride {
  permission: string
  effect: 'grant' | 'revoke'
}

interface RolePermissionLayer {
  role: string
  baseline: string[]
  overrides: PermissionOverride[]
  effective: string[]
  locked: boolean
  can_edit: boolean
}

interface RolePermissionMatrix {
  version: number
  groups: PermissionGroup[]
  catalog: PermissionEntry[]
  roles: RolePermissionLayer[]
}

interface UserPermissionView {
  version: number
  user: KunUser
  roles: string[]
  baseline: string[]
  overrides: PermissionOverride[]
  effective: string[]
  locked: boolean
  can_edit: boolean
}

interface PermissionOverriddenUser {
  user: KunUser
  overrides: PermissionOverride[]
}

interface PermissionChange {
  id: number
  actor: KunUser
  target_role: string | null
  target_user: KunUser | null
  before: PermissionOverride[]
  after: PermissionOverride[]
  reason: string
  created: string
}
