import { kunMoyuMoe } from '~/config/moyu-moe'

type AvatarSource = {
  avatar?: string | null
  avatar_image_hash?: string | null
}

const IMAGE_BED = kunMoyuMoe.domain.imageBed.replace(/\/$/, '')

const isHexHash = (s: string): boolean => /^[0-9a-f]{4,}$/i.test(s)

const buildImageServiceURL = (hash: string, variant?: string): string => {
  if (hash.length < 4 || !isHexHash(hash)) return ''
  const shard1 = hash.slice(0, 2)
  const shard2 = hash.slice(2, 4)
  const suffix = variant ? `_${variant}` : ''
  return `${IMAGE_BED}/${shard1}/${shard2}/${hash}${suffix}.webp`
}

// The hash is the avatar. A bare `avatar` is only drawn as-is: since infra #380
// it is the hash URL itself, and rewriting it to the legacy `-100.webp` variant
// names an object image_service never stored.
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
  return (user.avatar ?? '').trim()
}
