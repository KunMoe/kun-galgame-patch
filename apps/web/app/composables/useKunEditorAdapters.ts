import type { KunEditorAdapters, NotifyLevel } from '@kungal/editor-core'
import { resolveAvatarUrl } from '~/shared/utils/resolveAvatarUrl'
import { getRandomSticker } from '~/shared/utils/getRandomSticker'

// Host policy injected into the shared @kungal/editor-vue <KunEditor>. The editor
// owns the mechanism (ProseMirror schema, plugins, dual view); moyu supplies WHERE
// uploads go, HOW @mentions resolve, WHAT stickers exist, HOW toasts show. This
// replaces the in-repo components/kun/milkdown/ port (deleted with the migration).

// ── @mention URL policy — MUST mirror the backend contract ──────────────────
// moyu stores a mention as a real link `[@name](/user/<id>/resource)`; the server
// (internal/infrastructure/markdown/markdown.go) treats a `/user/<id>` link as a
// mention ONLY when its text starts with `@` (so ordinary links to a user page
// stay plain links) and pulls the uid for notifications from that same form.
// `mentionFromUrl` reproduces that `@` guard via the link text kun-editor passes.
const mentionToUrl = (userId: number) => `/user/${userId}/resource`
const mentionFromUrl = (url: string, text: string): number | null => {
  if (!text.startsWith('@')) return null
  const m = url.match(/^\/user\/(\d+)/)
  return m ? Number(m[1]) : null
}

// ── image_service upload (preset `topic`) → domain-agnostic token /image/<hash> ──
// We store the TOKEN, not an absolute CDN URL (image_service 契约 04): the server
// resolves it to the CDN at render time, and the web /image/:hash 302 route
// resolves it for the editor preview. Throw on failure so the editor drops the
// in-flight upload placeholder.
const uploadEditorImage = async (apiBase: string, file: File): Promise<string> => {
  const formData = new FormData()
  formData.append('preset', 'topic')
  formData.append('file', file, file.name)
  const res = await $fetch<{
    code: number
    message: string
    data: { hash: string } | null
  }>(`${apiBase}/upload/image-service`, {
    method: 'POST',
    body: formData,
    credentials: 'include'
  })
  if (res.code !== 0 || !res.data) {
    throw new Error(res.message || '图片上传失败')
  }
  return `/image/${res.data.hash}`
}

// ── sticker source — sticker.kungal.com sets KUNgal1..7 (see the old _stickers.ts) ──
const STICKER_BASE = 'https://sticker.kungal.com/stickers'
const SET_SIZES = [80, 80, 80, 80, 80, 80, 18]
const stickerPacks = SET_SIZES.map((size, setIndex) => ({
  name: `KUNgal${setIndex + 1}`,
  stickers: Array.from({ length: size }, (_, i) => ({
    src: `${STICKER_BASE}/KUNgal${setIndex + 1}/${i + 1}.webp`,
    name: `KUNgal${setIndex + 1}-${i + 1}`
  }))
}))

interface SearchUser {
  id: number
  name: string
  avatar?: string
  avatar_image_hash?: string
}

/**
 * Build the moyu adapter bundle for <KunEditor>. Pass `{ image: false }` for the
 * galgame 简介 editor — omitting `uploadImage` + `stickerSource` is how kun-editor
 * drops every image affordance (the old `allow-image=false` flag).
 */
export const useKunEditorAdapters = (
  options: { image?: boolean } = {}
): KunEditorAdapters => {
  const { image = true } = options
  const config = useRuntimeConfig()
  const apiBase =
    (config.public.apiBase as string) || 'http://127.0.0.1:5214/api/v1'

  const searchMentionUsers = async (query: string) => {
    const res = await $fetch<{ code: number; data: SearchUser[] }>(
      `${apiBase}/user/search?query=${encodeURIComponent(query)}`,
      { credentials: 'include' }
    )
    const users = res?.code === 0 ? (res.data ?? []) : []
    // kun-editor's dropdown renders <img :src="avatar"> as-is, so resolve the
    // hash-addressed avatar to a ready URL here (image_service variant `100`).
    return users.map((u) => ({
      id: u.id,
      name: u.name,
      avatar: resolveAvatarUrl(u, '100') || getRandomSticker(u.name).value
    }))
  }

  const notify = (message: string, level: NotifyLevel) => {
    useKunMessage(message, level)
  }

  return {
    searchMentionUsers,
    mentionToUrl,
    mentionFromUrl,
    notify,
    ...(image
      ? {
          uploadImage: (file: File) => uploadEditorImage(apiBase, file),
          stickerSource: () => stickerPacks
        }
      : {})
  }
}
