import { ref, type Ref } from 'vue'
import { randomNum } from '../../shared/utils/random'

const KUN_STICKER_DOMAIN = 'https://sticker.kungal.com'

const makeUrl = (): string => {
  const randomPackIndex = randomNum(1, 5)
  const randomStickerIndex = randomNum(1, 80)
  return `${KUN_STICKER_DOMAIN}/stickers/KUNgal${randomPackIndex}/${randomStickerIndex}.webp`
}

// Client-only cache.
//
// A module-level Map on the SERVER side would persist across HTTP
// requests (the Nuxt SSR process is long-lived), but NUXT.payload is
// per-request — so a cache hit on request N would skip useState
// entirely, the payload for request N would have no entry for this
// key, and the client would re-run the init fn and pick a different
// URL → hydration mismatch on every page load after the first.
//
// Restricting the Map to the client side keeps the useful invariant
// (same sticker for the same id across reactive recomputes within a
// session) without breaking SSR payload serialization.
const clientCache = import.meta.client
  ? new Map<string, Ref<string>>()
  : null

export const getRandomSticker = (id: string): Ref<string> => {
  const key = `random-sticker-${id}`

  if (clientCache) {
    const existing = clientCache.get(key)
    if (existing) {
      return existing
    }
  }

  const nuxtApp = tryUseNuxtApp()
  let stickerUrl: Ref<string>
  if (nuxtApp) {
    // Inside Nuxt context — useState wires the URL into NUXT.payload so
    // server-picked value is what the client hydrates with. Per-request
    // scoped on the server, so there's no leakage between requests.
    stickerUrl = useState<string>(key, makeUrl)
  } else {
    // Reactive recompute path on the client (e.g. a `<KunAvatar :user>`
    // inside a computed that re-evaluates after a parent's
    // useFetch().refresh()) — tryUseNuxtApp returns null in those
    // microtasks and calling useState here would crash with
    // "Cannot read properties of null (reading '$nuxt')". The id is
    // by definition post-hydration (the SSR pass never saw it) so no
    // hydration parity exists to preserve.
    stickerUrl = ref(makeUrl())
  }

  if (clientCache) {
    clientCache.set(key, stickerUrl)
  }
  return stickerUrl
}
