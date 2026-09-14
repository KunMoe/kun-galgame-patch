interface UpstreamSticker {
  src: string
  name: string
  hash?: string
}

interface StickerItem {
  src: string
  name: string
}

interface StickerPack {
  name: string
  stickers: StickerItem[]
}

/**
 * The official sticker packs, proxied from sticker.kungal.com.
 *
 * The picker's list used to be built here in the browser from
 * `sticker.kungal.com/stickers/KUNgal{set}/{n}.webp` and a hardcoded
 * `[80,80,80,80,80,80,18]`. Every one of those 498 URLs 404s today: the path
 * addressed a position in a collection on a site we do not own, and that site
 * stopped serving static files.
 *
 * `src` is rewritten to `/image/<hash>_<variant>` before it reaches the picker,
 * and that is the load-bearing line here. Whatever `src` holds is what the
 * picker writes into the post: upstream hands over an absolute CDN URL, so
 * passing it through welded `image.kungal.iloveren.link` into every message and
 * comment that used a sticker — the same mistake one domain later, which is how
 * the first 498 URLs died. The token names the bytes; /image/** resolves it to
 * whichever CDN is configured, and the picker's own thumbnails go through that
 * route too.
 *
 * Server-side because the browser must not reach across origins for it, and
 * cached because 82 KB is worth fetching once an hour rather than per picker
 * open. `staleMaxAge` is the load-bearing part: a sticker site outage should
 * cost last week's packs, not an empty picker.
 */
export default defineCachedEventHandler(
  async (): Promise<{ packs: StickerPack[] }> => {
    const base = useRuntimeConfig().stickerBaseUrl
    const res = await $fetch<{
      code: number
      message: string
      data: {
        variant?: string
        packs: { name: string; stickers: UpstreamSticker[] }[]
      } | null
    }>(`${base}/api/v1/editor-packs`, { timeout: 8000 })
    if (res.code !== 0 || !res.data) {
      throw createError({
        statusCode: 502,
        statusMessage: res.message || 'sticker packs unavailable'
      })
    }
    const variant = res.data.variant ?? ''
    return {
      packs: res.data.packs.map((pack) => ({
        name: pack.name,
        stickers: pack.stickers.map((sticker) => ({
          name: sticker.name,
          src: sticker.hash
            ? `/image/${sticker.hash}${variant ? `_${variant}` : ''}`
            : sticker.src
        }))
      }))
    }
  },
  { name: 'sticker-packs', maxAge: 3600, staleMaxAge: 604800, swr: true }
)
