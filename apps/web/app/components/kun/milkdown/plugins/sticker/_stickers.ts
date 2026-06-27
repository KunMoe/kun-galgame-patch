// Single source of truth for the sticker URLs, used by the milkdown editor
// sticker plugin (Container.vue) — mirrors how plugins/emoji/_isoEmoji.ts is the
// shared source for emoji.
//
// sticker.kungal.com layout (verified 2026-06-25 via HEAD requests): sets
// KUNgal1..7; sets 1-6 hold 80 stickers each, set 7 (the latest) currently holds
// 18. Listing EXACTLY the existing ids means no 404 (no blank tiles at the end)
// AND the newest set shows. When the host grows, bump the matching SET_SIZES
// entry (e.g. 18 → 80) or append the next set's size.
const STICKER_BASE = 'https://sticker.kungal.com/stickers'
const SET_SIZES = [80, 80, 80, 80, 80, 80, 18]

export const stickerArray: string[] = SET_SIZES.flatMap((size, setIndex) =>
  Array.from(
    { length: size },
    (_, i) => `${STICKER_BASE}/KUNgal${setIndex + 1}/${i + 1}.webp`
  )
)
