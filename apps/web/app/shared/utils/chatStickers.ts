// Full sticker set used by the chat emoji/sticker picker. Mirrors the
// next-web generateStickerArray(): 6 packs × 80, minus the last 6 that 404.
export const chatStickerArray = (): string[] => {
  const result: string[] = []
  for (let set = 1; set <= 6; set++) {
    for (let id = 1; id <= 80; id++) {
      result.push(`https://sticker.kungal.com/stickers/KUNgal${set}/${id}.webp`)
    }
  }
  return result.slice(0, -6)
}
