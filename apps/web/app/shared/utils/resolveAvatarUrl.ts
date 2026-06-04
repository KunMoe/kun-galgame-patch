import { kunMoyuMoe } from '~/config/moyu-moe'

// resolveAvatarUrl picks the best URL for a user's avatar.
//
// Per docs/oauth/api-reference.md, the OAuth /users/batch brief now returns
// both:
//   - `avatar`            — legacy absolute URL (e.g. https://image.kungal.com/avatar/user_30/avatar.webp)
//   - `avatar_image_hash` — hash-addressed reference into the central image_service
//
// During the image_service rollout both fields coexist. We prefer the hash
// when present, falling back to the legacy URL. When neither is set, the
// caller is responsible for showing a placeholder (KunAvatar uses a sticker).
//
// The hash → URL convention follows image_service §02 (`{cdn}/ab/cd/<hash>.webp`,
// no `/img/` segment; shard prefix = first 2 + next 2 hex chars). Variants (`_100`) are used for
// list-density avatars to cut bandwidth.

type AvatarSource = {
  avatar?: string | null
  avatar_image_hash?: string | null
}

const IMAGE_BED = kunMoyuMoe.domain.imageBed.replace(/\/$/, '')

const isHexHash = (s: string): boolean => /^[0-9a-f]{4,}$/i.test(s)

const buildImageServiceURL = (hash: string, variant?: string): string => {
  // Shard by first two pairs of hex chars to match the image_service object key
  // layout. Hashes shorter than 4 chars are sentinel/placeholder values and
  // are skipped (caller falls back to legacy URL).
  if (hash.length < 4 || !isHexHash(hash)) return ''
  const shard1 = hash.slice(0, 2)
  const shard2 = hash.slice(2, 4)
  const suffix = variant ? `_${variant}` : ''
  return `${IMAGE_BED}/${shard1}/${shard2}/${hash}${suffix}.webp`
}

// Convert the legacy *.webp URL to its small variant (*-100.webp). Used when
// rendering list-density avatars; for full-size avatars caller passes
// `variant: undefined`.
const toLegacyVariant = (url: string, variant?: string): string => {
  if (!variant) return url
  return url.replace(/\.webp$/, `-${variant}.webp`)
}

export const resolveAvatarUrl = (
  user: AvatarSource | null | undefined,
  variant?: '100'
): string => {
  if (!user) return ''
  const hash = (user.avatar_image_hash ?? '').trim()
  if (hash) {
    const u = buildImageServiceURL(hash, variant)
    if (u) return u
  }
  const legacy = (user.avatar ?? '').trim()
  if (!legacy) return ''
  return toLegacyVariant(legacy, variant)
}
