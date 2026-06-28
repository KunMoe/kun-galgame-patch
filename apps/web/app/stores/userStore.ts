import { defineStore } from 'pinia'

// Mirrors GET /api/v1/auth/me (and /oauth/callback) MeResponse. Keys are
// snake_case to match the wire format verbatim; no client-side remapping.
//
// `roles` is the OAuth-side role set ("admin", "moderator", "user", ...).
// After the OAuth migration, per-site numeric role (1/2/3/4) is no longer
// returned by the backend; downstream gates (isAdmin / isModerator) read
// this array directly.
export interface UserState {
  // DB PK (Prisma user.id == Go MeResponse.id) — matches /user/:id,
  // /ranking/user, and @kungal/ui-core's KunUser. The legacy `uid` transport
  // label was hard-cut to `id` everywhere (JWT claim, URL routes, Fiber
  // session JSON). See apps/api/internal/auth/dto/dto.go.
  id: number
  // OAuth subject UUID (MeResponse.sub). Stable per identity and used as the
  // `login_hint` when switching back to this account. See useKnownAccounts.
  sub: string
  name: string
  avatar: string
  // OAuth image_service hash for the avatar; preferred by resolveAvatarUrl
  // over `avatar` once the image_service is live. See docs/oauth/api-reference.md.
  avatar_image_hash: string
  bio: string
  moemoepoint: number
  roles: string[]

  daily_check_in: number
  daily_image_count: number
  daily_upload_size: number

  muted_message_types: string[]
}

const initialUserState: UserState = {
  id: 0,
  sub: '',
  name: '',
  avatar: '',
  avatar_image_hash: '',
  bio: '',
  moemoepoint: 0,
  roles: [],
  daily_check_in: 1,
  daily_image_count: 0,
  daily_upload_size: 0,
  muted_message_types: []
}

export const useUserStore = defineStore('user', {
  state: (): { user: UserState } => ({
    user: { ...initialUserState }
  }),
  actions: {
    setUser(user: Partial<UserState>) {
      this.user = {
        ...this.user,
        ...user,
        // A backend `roles: null` (nil []string) or a stale/legacy cookie must
        // not poison the store: isAdmin/isModerator call roles.includes() during
        // SSR. Coerce to an array so we never persist null.
        roles: user.roles ?? this.user.roles ?? [],
        muted_message_types: this.user.muted_message_types
      }
    },
    toggleMutedMessageType(type: string) {
      const muted = this.user.muted_message_types
      this.user.muted_message_types = muted.includes(type)
        ? muted.filter((t) => t !== type)
        : [...muted, type]
    },
    logout() {
      this.user = {
        ...initialUserState,
        muted_message_types: this.user.muted_message_types
      }
    }
  },
  getters: {
    isLoggedIn: (state) => state.user.id > 0 && !!state.user.name,
    // OAuth role mapping (see docs/user-migration/02-data-mapping.md §7):
    //   moyu super-admin -> "admin"
    //   moyu/kungal admin -> "moderator"
    // The backend's middleware.RequireRole("admin", "moderator") matches the
    // isModerator getter; admin-only gates use isAdmin.
    // `?? []` guards against a null roles surviving in an old persisted cookie
    // (pre-fix sessions): without it these getters throw "Cannot read
    // properties of null (reading 'includes')" during SSR of /patch/* pages.
    // `ren` (莲) is a DB-preset super-admin and is treated exactly like `admin`
    // everywhere in moyu (mirrors the backend middleware.SuperAdminRoles).
    isAdmin: (state) =>
      (state.user.roles ?? []).includes('admin') ||
      (state.user.roles ?? []).includes('ren'),
    isModerator: (state) =>
      (state.user.roles ?? []).includes('admin') ||
      (state.user.roles ?? []).includes('ren') ||
      (state.user.roles ?? []).includes('moderator'),
    // `creator` is OAuth's trusted-publisher tier (wiki direct-publish), and on
    // moyu it also shares the moderator upload allowance (5GB / 100GB daily).
    // It is NOT a moderation role, so it is deliberately separate from
    // isModerator (no admin-panel / mod-action access).
    isCreator: (state) => (state.user.roles ?? []).includes('creator'),
    // Ad-free = holds any role other than "user" (creator / moderator / admin).
    // Anonymous (no roles) and plain users still see ads; every elevated role is
    // exempt. Drives the AIEro* ad gates.
    isAdFree: (state) => (state.user.roles ?? []).some((r) => r !== 'user')
  },
  // Cookie-backed persistence is intentional: the cookie is sent on the
  // initial HTML request so the SSR pass already has the logged-in user
  // available (name / avatar / roles / counts). Without this the page would
  // render anonymously on the server and only "fill in" after onMounted on
  // the client, producing a visible flicker. cookieOptions (maxAge 7d,
  // sameSite=lax — Lax not Strict so the cookie survives a cross-site link
  // click into a detail page; see nuxt.config.ts for the full rationale) come
  // from the global piniaPluginPersistedstate config in nuxt.config.ts.
  persist: {
    key: 'kun-patch-user-store',
    storage: piniaPluginPersistedstate.cookies()
  }
})
