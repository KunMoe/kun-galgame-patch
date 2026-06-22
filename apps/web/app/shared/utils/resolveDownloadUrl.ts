import { kunMoyuMoe } from '~/config/moyu-moe'

// resolveDownloadLinks turns a patch_resource's `content` field into the list of
// usable download URLs the UI renders.
//
// Storage semantics (mirror the apps/api CreateResource invariant):
//   - storage="s3":   `content` is the bare s3_key (object path, e.g.
//                     "patch/6924/<hash>/file.7z"). The public download URL is
//                     `{domain.storage}/{key}` — the B2 bucket fronted by
//                     oss.moyu.moe. The frontend prepends the domain here, the
//                     same way resolveAvatarUrl / resolveBannerUrl prepend
//                     domain.imageBed for image_service assets. The backend
//                     intentionally returns the bare key so swapping the
//                     download CDN/domain is a single frontend-config change
//                     with no DB backfill.
//   - storage="user": `content` is the user's own comma-separated full links;
//                     used verbatim (never prefixed).
//
// The s3_key is concatenated raw (not percent-encoded) so the browser performs
// the encoding on navigation — matching the previous backend behavior and
// avoiding double-encoding of non-ASCII filenames.

const STORAGE_BASE = kunMoyuMoe.domain.storage.replace(/\/$/, '')

export const resolveDownloadLinks = (
  storage: string | undefined | null,
  content: string | undefined | null,
  // Artifact-backed rows carry a ready, absolute URL resolved by the backend
  // (storage="s3" but content is empty) — use it directly, no domain prefixing.
  downloadUrl?: string | undefined | null
): string[] => {
  if (downloadUrl) return [downloadUrl]
  const parts = (content ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  if (storage !== 's3') return parts
  return parts.map((key) =>
    // Defensive: a row that already carries an absolute URL (legacy/user data
    // mislabeled as s3) is left as-is rather than double-prefixed.
    /^https?:\/\//i.test(key) ? key : `${STORAGE_BASE}/${key}`
  )
}
