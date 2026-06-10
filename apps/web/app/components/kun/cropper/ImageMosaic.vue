<script setup lang="ts">
// 封面打码（马赛克）编辑器 — ported from the legacy next-web KunImageMosaicModal.
//
// Paint over regions (e.g. NSFW areas) to pixelate them. Pure canvas, no deps:
//   - precompute a fully-pixelated copy of the image (pixelate()),
//   - per brush stroke, mask that copy to the stroke path via
//     globalCompositeOperation='destination-in', then composite it onto the
//     display canvas, which ACCUMULATES every stroke.
// The display canvas is the natural image resolution (so the export is full-res)
// and CSS-scaled to fit; pointer coords are scaled back to canvas space.
const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ src: string }>()
const emit = defineEmits<{ complete: [blob: Blob] }>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const blockSize = ref(24)

let image: HTMLImageElement | null = null
let originalData: ImageData | null = null // clean pixels (reset + re-pixelate source)
let mosaicCanvas: HTMLCanvasElement | null = null // offscreen pixelated, masked per stroke
let mosaicData: ImageData | null = null // pixelated copy at the current block size
let drawing = false
let strokeStart = true

// Average each blockSize×blockSize block → a pixelated ImageData.
const pixelate = (src: ImageData, size: number): ImageData => {
  const { width: w, height: h, data: s } = src
  const out = new ImageData(w, h)
  const d = out.data
  for (let by = 0; by < h; by += size) {
    for (let bx = 0; bx < w; bx += size) {
      let r = 0
      let g = 0
      let b = 0
      let a = 0
      let n = 0
      for (let y = by; y < by + size && y < h; y++) {
        for (let x = bx; x < bx + size && x < w; x++) {
          const i = (y * w + x) * 4
          r += s[i]
          g += s[i + 1]
          b += s[i + 2]
          a += s[i + 3]
          n++
        }
      }
      r /= n
      g /= n
      b /= n
      a /= n
      for (let y = by; y < by + size && y < h; y++) {
        for (let x = bx; x < bx + size && x < w; x++) {
          const i = (y * w + x) * 4
          d[i] = r
          d[i + 1] = g
          d[i + 2] = b
          d[i + 3] = a
        }
      }
    }
  }
  return out
}

const rebuildMosaicData = () => {
  if (originalData) mosaicData = pixelate(originalData, blockSize.value)
}

const resetCanvas = () => {
  const canvas = canvasRef.value
  if (canvas && originalData) canvas.getContext('2d')?.putImageData(originalData, 0, 0)
}

const setup = () => {
  const canvas = canvasRef.value
  if (!canvas || !props.src) return
  const img = new Image()
  img.onload = () => {
    // The modal may have been closed (teardown) before this async decode
    // resolved — bail so we don't draw to an unmounted canvas or revive
    // torn-down state. mosaicSrc is a same-origin object URL, so no taint.
    if (image !== img || canvasRef.value !== canvas) return
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.drawImage(img, 0, 0)
    originalData = ctx.getImageData(0, 0, canvas.width, canvas.height)
    mosaicCanvas = document.createElement('canvas')
    mosaicCanvas.width = canvas.width
    mosaicCanvas.height = canvas.height
    rebuildMosaicData()
  }
  // Set `image` before `src` so the onload guard above can identify a stale load.
  image = img
  img.src = props.src
}

const teardown = () => {
  if (image) image.onload = null
  image = null
  originalData = null
  mosaicData = null
  mosaicCanvas = null
  drawing = false
}

watch(open, async (v) => {
  if (!import.meta.client) return
  if (v) {
    await nextTick()
    setup()
  } else {
    teardown()
  }
})

watch(blockSize, () => rebuildMosaicData())

const toCanvasXY = (e: PointerEvent) => {
  const canvas = canvasRef.value!
  const rect = canvas.getBoundingClientRect()
  return {
    x: ((e.clientX - rect.left) * canvas.width) / rect.width,
    y: ((e.clientY - rect.top) * canvas.height) / rect.height
  }
}

const onPointerDown = (e: PointerEvent) => {
  if (!canvasRef.value) return
  canvasRef.value.setPointerCapture(e.pointerId)
  drawing = true
  strokeStart = true
}

const onPointerMove = (e: PointerEvent) => {
  if (!drawing || !canvasRef.value || !mosaicCanvas || !mosaicData) return
  const octx = canvasRef.value.getContext('2d')
  const mctx = mosaicCanvas.getContext('2d')
  if (!octx || !mctx) return
  // Refill the mosaic canvas with the full pixelated copy, then keep only the
  // brush path (destination-in), then composite that onto the display canvas.
  mctx.globalCompositeOperation = 'source-over'
  mctx.putImageData(mosaicData, 0, 0)
  mctx.globalCompositeOperation = 'destination-in'
  const { x, y } = toCanvasXY(e)
  if (strokeStart) {
    mctx.beginPath()
    mctx.moveTo(x, y)
    strokeStart = false
  }
  mctx.lineTo(x, y)
  mctx.lineWidth = blockSize.value
  mctx.lineCap = 'round'
  mctx.lineJoin = 'round'
  mctx.strokeStyle = '#000'
  mctx.stroke()
  octx.drawImage(mosaicCanvas, 0, 0)
}

const onPointerUp = () => {
  drawing = false
}

const complete = async () => {
  const canvas = canvasRef.value
  if (!canvas) return
  const blob = await new Promise<Blob | null>((resolve) =>
    canvas.toBlob((b) => resolve(b), 'image/webp', 0.9)
  )
  if (!blob) {
    useKunMessage('打码失败', 'error')
    return
  }
  emit('complete', blob)
  open.value = false
}
</script>

<template>
  <KunModal
    v-model="open"
    inner-class-name="max-w-3xl w-full"
    :is-dismissable="false"
  >
    <div class="space-y-4">
      <h3 class="text-lg font-semibold">封面打码</h3>
      <p class="text-default-500 text-sm">
        在需要遮挡的区域涂抹即可打码，可多次涂抹多个区域；拖动下方滑块调整马赛克粗细。
      </p>

      <div
        class="border-default-200 max-h-[60vh] overflow-auto rounded-lg border"
      >
        <canvas
          ref="canvasRef"
          aria-label="封面打码画布：在需要遮挡的区域涂抹打码"
          class="block max-w-full cursor-crosshair touch-none select-none"
          @pointerdown="onPointerDown"
          @pointermove="onPointerMove"
          @pointerup="onPointerUp"
          @pointercancel="onPointerUp"
        />
      </div>

      <div class="flex items-center gap-3">
        <span class="shrink-0 text-sm">马赛克粗细</span>
        <input
          v-model.number="blockSize"
          type="range"
          min="16"
          max="48"
          step="1"
          class="accent-primary h-1 flex-1 cursor-pointer"
        />
        <span class="text-default-500 w-10 shrink-0 text-right text-sm tabular-nums">
          {{ blockSize }}px
        </span>
      </div>

      <div class="flex justify-end gap-2">
        <KunButton variant="light" color="default" @click="resetCanvas">
          <KunIcon name="lucide:eraser" class="size-4" />
          重置
        </KunButton>
        <KunButton variant="light" color="danger" @click="open = false">
          取消
        </KunButton>
        <KunButton color="primary" @click="complete">
          <KunIcon name="lucide:check" class="size-4" />
          完成
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
