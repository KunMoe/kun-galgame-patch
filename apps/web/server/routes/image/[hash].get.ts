// 302 fallback for domain-agnostic content image tokens (image_service 契约 04).
//
// User content stores `/image/<hash>[_variant]` instead of an absolute CDN URL
// so a domain change is one config edit. The FAST path is server-side: goldmark
// rewrites the token to a full CDN URL when it renders comments/notes to HTML.
// This route is the FALLBACK for everything not rendered through goldmark — the
// editor preview, the sticker picker's own thumbnails, raw markdown, RSS,
// external consumers — anything that loads `<img src="/image/<hash>">` against
// the web origin. It 302s to the same `{imageBed}/<aa>/<bb>/<hash>[_variant].webp`
// object path the rest of the app builds (imageclient.VariantURL /
// resolveBannerUrl's imageServiceUrl).
//
// The variant suffix is not optional decoration: stickers are stored as `_320`
// and a route that accepted only a bare hash answered 404 for every one of them.
const TOKEN = /^([0-9a-f]{64})(?:_([a-z0-9]+))?$/

export default defineEventHandler((event) => {
  const match = TOKEN.exec(getRouterParam(event, 'hash') ?? '')
  if (!match) {
    throw createError({ statusCode: 404, statusMessage: 'Not Found' })
  }
  const [, hash, variant] = match
  const base = useRuntimeConfig().public.imageBed.replace(/\/$/, '')
  const suffix = variant ? `_${variant}` : ''
  const url = `${base}/${hash.slice(0, 2)}/${hash.slice(2, 4)}/${hash}${suffix}.webp`
  // 302 (not 301): the token→URL mapping is config-driven and may change if the
  // CDN domain moves, so it must not be permanently cached by clients.
  return sendRedirect(event, url, 302)
})
