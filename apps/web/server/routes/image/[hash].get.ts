// 302 fallback for domain-agnostic content image tokens (image_service 契约 04).
//
// User content stores `/image/<hash>` instead of an absolute CDN URL so a domain
// change is one config edit. The FAST path is server-side: goldmark rewrites the
// token to a full CDN URL when it renders comments/notes to HTML. This route is
// the FALLBACK for everything not rendered through goldmark — the milkdown editor
// preview, raw markdown, RSS, external consumers — anything that loads
// `<img src="/image/<hash>">` against the web origin. It 302s to the same
// `{imageBed}/<aa>/<bb>/<hash>.webp` object path the rest of the app builds
// (imageclient.MainURL / resolveBannerUrl's imageServiceUrl).
export default defineEventHandler((event) => {
  const hash = getRouterParam(event, 'hash') ?? ''
  if (!/^[0-9a-f]{64}$/.test(hash)) {
    throw createError({ statusCode: 404, statusMessage: 'Not Found' })
  }
  const base = useRuntimeConfig().public.imageBed.replace(/\/$/, '')
  const url = `${base}/${hash.slice(0, 2)}/${hash.slice(2, 4)}/${hash}.webp`
  // 302 (not 301): the token→URL mapping is config-driven and may change if the
  // CDN domain moves, so it must not be permanently cached by clients.
  return sendRedirect(event, url, 302)
})
