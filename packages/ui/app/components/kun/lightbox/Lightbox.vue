<script setup lang="ts">
import { useBodyScrollLock } from '../../../composables/useBodyScrollLock'

interface ImageItem {
  src: string
  alt?: string
}

const props = defineProps<{
  images: ImageItem[]
  isOpen: boolean
  initialIndex?: number
}>()

const emit = defineEmits<{
  'update:isOpen': [value: boolean]
}>()

// Modern <dialog>-based lightbox. Major capabilities the browser handles
// for us (no JS needed):
//   - focus trap inside the modal
//   - ESC key closes
//   - background <body> becomes inert (screen readers skip it)
//   - ::backdrop pseudo-element for the overlay
//
// What we still own:
//   - body-scroll lock (native <dialog> does NOT lock body scroll)
//   - ←/→ navigation (scoped to open state)
//   - click-on-backdrop-to-close (native dialog fires click with target
//     === dialog element when the backdrop is clicked)
//   - all the image-viewer mechanics: wheel zoom, drag/pan, pinch, swipe,
//     double-click zoom, download
//
// View Transitions API: when supported (Chrome/Edge 111+, Safari 18+),
// we wrap state mutations in `document.startViewTransition` so opening
// /closing / changing index get a free cross-fade. Consumers can opt
// into the full thumbnail→hero morph by giving their trigger image a
// matching `view-transition-name: kun-lightbox-image` CSS rule.
const MIN_SCALE = 1
const MAX_SCALE = 5
const SWIPE_THRESHOLD = 50
// Single zoom step for the toolbar +/− buttons. Matches the wheel step
// in handleWheel below so the two paths feel consistent.
const ZOOM_STEP = 0.5

const currentIndex = ref(props.initialIndex || 0)
const scale = ref(1)
// True while a 2-finger pinch is in progress. Drives both gesture
// dispatch (avoid running single-touch drag math during pinch) and the
// CSS transition (zero-latency during pinch — see transformStyle).
const isPinching = ref(false)
// Manual double-tap state for touchscreens: `@dblclick` is fired by the
// browser only when its built-in tap-to-click synthesis kicks in, which
// is unreliable on iOS Safari + sometimes on Android Chrome. We detect
// double-tap ourselves in onTouchEnd by comparing the gap between two
// successive single-touch ends.
let lastTapTime = 0
let lastTapX = 0
let lastTapY = 0
const DOUBLE_TAP_MS = 300
const DOUBLE_TAP_PX = 30
// Rotation is stored unbounded (±∞) on purpose. The user-visible state
// is always one of 4 orientations (0/90/180/270°) because every 90°
// increment cycles through them visually — but the INTERNAL number
// needs to keep growing/shrinking so CSS `transition: transform` always
// interpolates the SHORT path the user just clicked (90° in their
// chosen direction), never the wrap-around long way (e.g. 270°→0° going
// 270° backwards). resetTransform snaps to the nearest 360-multiple to
// avoid the number accumulating indefinitely across reset cycles.
const rotation = ref(0)
const position = reactive({ x: 0, y: 0 })
const isDragging = ref(false)
const dragStart = reactive({ x: 0, y: 0 })
const lastTouch = reactive({ x: 0, y: 0 })
const lastTouchDistance = ref(0)
const dragStartTime = ref(0)
const initialDragPosition = reactive({ x: 0, y: 0 })

// Direction of the most recent prev/next call. Drives which slide
// transition class set Vue applies on the next image swap — 'slide-next'
// for forward (old left, new in from right), 'slide-prev' for backward.
const slideDir = ref<'slide-next' | 'slide-prev'>('slide-next')

const dialogRef = ref<HTMLDialogElement | null>(null)
const imageRef = ref<HTMLImageElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)

const currentImage = computed(() => props.images[currentIndex.value]?.src || '')
const currentAlt = computed(() => props.images[currentIndex.value]?.alt || '')

const transformStyle = computed(() => ({
  transform: `translate3d(${position.x}px, ${position.y}px, 0) scale(${scale.value}) rotate(${rotation.value}deg)`,
  // Disable transition during continuous gestures (drag / pinch) so the
  // image tracks the finger / cursor frame-for-frame. With the 0.3s
  // ease-out left on during pinch, the image lags ~5 frames behind the
  // pinch midpoint and feels "stuck in syrup". Discrete actions (wheel
  // step, button click, double-click) keep the transition for the
  // smooth zoom-in feel.
  transition:
    isDragging.value || isPinching.value
      ? 'none'
      : 'transform 0.3s ease-out'
}))

const resetTransform = () => {
  // Snap rotation to its NEAREST 360-multiple instead of literal 0.
  // Visual result is identical (every 360-multiple looks like default
  // orientation), but the CSS transition animates ≤180° back to the
  // snap target rather than unwinding every full turn the user clicked.
  // E.g. rotation=720 → snap to 720 (no-op, already default-looking);
  //      rotation=810 (= 90° visual) → snap to 720 (90° CCW animation).
  rotation.value = Math.round(rotation.value / 360) * 360
  scale.value = MIN_SCALE
  position.x = 0
  position.y = 0
}

// Toolbar actions. All operate in the SAME state as wheel/touch/double-
// click — the toolbar is just a discoverability layer, not a separate
// mode. Zoom around the geometric center (not the cursor position) since
// there's no event coordinates to anchor to.
const zoomIn = () => {
  const next = Math.min(MAX_SCALE, scale.value + ZOOM_STEP)
  if (next === scale.value) return
  scale.value = next
  const c = constrainPosition(position.x, position.y)
  position.x = c.x
  position.y = c.y
}
const zoomOut = () => {
  const next = Math.max(MIN_SCALE, scale.value - ZOOM_STEP)
  if (next === scale.value) return
  scale.value = next
  const c = constrainPosition(position.x, position.y)
  position.x = c.x
  position.y = c.y
}

// Rotate 90° per click. No modulo on the stored value — see the
// `rotation` declaration comment for why (CSS transition direction).
// Clicks are unlimited; visually the image cycles through 4 orientations.
//
// Note: at 90°/270° (mod 360) a wide image's bounding box exceeds the
// container — CSS transform doesn't affect layout. User can pan or
// reset to recover.
const rotateRight = () => {
  rotation.value += 90
}
const rotateLeft = () => {
  rotation.value -= 90
}

// startViewTransition wrapper that degrades to direct execution when the
// browser doesn't support the API (older Safari / Firefox stable). The
// callback runs synchronously either way.
const withViewTransition = (mutate: () => void) => {
  const doc = document as Document & {
    startViewTransition?: (cb: () => void) => unknown
  }
  if (typeof doc.startViewTransition === 'function') {
    doc.startViewTransition(mutate)
  } else {
    mutate()
  }
}

const { lock, unlock } = useBodyScrollLock()
let locked = false

// Sync the native <dialog>'s open state with the v-model prop. We use
// showModal() (not show()) so we get backdrop + inert + focus trap.
watch(
  () => props.isOpen,
  (open) => {
    const dlg = dialogRef.value
    if (!dlg) return
    if (open && !dlg.open) {
      // Reset on open so reuse of the same instance doesn't carry over
      // zoom/rotation/position from the previous session. Without this,
      // closing while zoomed and reopening on the same initialIndex
      // would start zoomed; the @load reset path that used to handle
      // this was removed along with the carousel transition rewrite.
      resetTransform()
      withViewTransition(() => dlg.showModal())
      if (!locked) {
        lock()
        locked = true
      }
    } else if (!open && dlg.open) {
      withViewTransition(() => dlg.close())
      if (locked) {
        unlock()
        locked = false
      }
    }
  },
  { flush: 'post' }
)

watch(
  () => props.initialIndex,
  () => {
    currentIndex.value = props.initialIndex || 0
    resetTransform()
  }
)

// Native dialog fires 'close' when ESC is pressed or close() runs. Use
// that to keep parent v-model in sync without our own ESC handler.
const onDialogClose = () => {
  if (props.isOpen) {
    emit('update:isOpen', false)
    resetTransform()
  }
  if (locked) {
    unlock()
    locked = false
  }
}

// Backdrop click: <dialog>'s click event fires with target === the dialog
// element itself when the user clicks outside the content. Our content
// fills the whole dialog so this only catches genuine misses (rare with
// fullscreen viewer, but covers the corners around a max-h/max-w image).
const onDialogClick = (e: MouseEvent) => {
  if (e.target === dialogRef.value) {
    emit('update:isOpen', false)
  }
}

const next = () => {
  // slideDir set BEFORE the index mutation so Vue's <Transition> picks
  // the right class set on the upcoming swap. Reset transform first so
  // the new slide enters with a clean state (a zoomed-out leave is
  // still visible alongside the centered enter — looks intentional).
  slideDir.value = 'slide-next'
  resetTransform()
  currentIndex.value = (currentIndex.value + 1) % props.images.length
}

const prev = () => {
  slideDir.value = 'slide-prev'
  resetTransform()
  currentIndex.value =
    (currentIndex.value - 1 + props.images.length) % props.images.length
}

const getBounds = () => {
  if (!imageRef.value || !containerRef.value) {
    return { minX: 0, maxX: 0, minY: 0, maxY: 0 }
  }
  const container = containerRef.value.getBoundingClientRect()
  const image = imageRef.value.getBoundingClientRect()
  const scaledWidth = image.width * scale.value
  const scaledHeight = image.height * scale.value
  const maxX = Math.max(0, (scaledWidth - container.width) / 2)
  const maxY = Math.max(0, (scaledHeight - container.height) / 2)
  return { minX: -maxX, maxX, minY: -maxY, maxY }
}

const constrainPosition = (x: number, y: number) => {
  if (scale.value <= 1) return { x: 0, y: 0 }
  const b = getBounds()
  return {
    x: Math.min(Math.max(x, b.minX), b.maxX),
    y: Math.min(Math.max(y, b.minY), b.maxY)
  }
}

const handleWheel = (e: WheelEvent) => {
  e.preventDefault()
  const delta = -e.deltaY
  const zoomFactor = 0.2
  const newScale = scale.value + (delta > 0 ? zoomFactor : -zoomFactor)
  const clampedScale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, newScale))

  if (clampedScale !== scale.value) {
    const rect = containerRef.value?.getBoundingClientRect()
    if (!rect) return
    const mouseX = e.clientX - rect.left
    const mouseY = e.clientY - rect.top
    const scaleChange = clampedScale / scale.value
    const newPosition = {
      x: mouseX - (mouseX - position.x) * scaleChange,
      y: mouseY - (mouseY - position.y) * scaleChange
    }
    const constrained = constrainPosition(newPosition.x, newPosition.y)
    position.x = constrained.x
    position.y = constrained.y
    scale.value = clampedScale
  }
}

const startDrag = (e: MouseEvent | TouchEvent) => {
  isDragging.value = true
  dragStartTime.value = Date.now()
  const point = 'touches' in e ? e.touches[0] : e
  dragStart.x = point!.clientX - position.x
  dragStart.y = point!.clientY - position.y
  initialDragPosition.x = point!.clientX
  initialDragPosition.y = point!.clientY
}

const onDrag = (e: MouseEvent | TouchEvent) => {
  if (!isDragging.value) return
  const point = 'touches' in e ? e.touches[0] : e
  if (scale.value <= 1) {
    position.x = point!.clientX - initialDragPosition.x
    return
  }
  const newPosition = {
    x: point!.clientX - dragStart.x,
    y: point!.clientY - dragStart.y
  }
  const constrained = constrainPosition(newPosition.x, newPosition.y)
  position.x = constrained.x
  position.y = constrained.y
}

const stopDrag = (e: MouseEvent | TouchEvent) => {
  if (!isDragging.value) return
  const point =
    'touches' in e ? (e as TouchEvent).changedTouches[0] : (e as MouseEvent)
  const deltaX = point!.clientX - initialDragPosition.x
  const deltaTime = Date.now() - dragStartTime.value
  const velocity = Math.abs(deltaX) / deltaTime

  if (
    scale.value <= 1 &&
    Math.abs(deltaX) > SWIPE_THRESHOLD &&
    velocity > 0.2
  ) {
    if (deltaX > 0) prev()
    else next()
    position.x = 0
    position.y = 0
  }
  isDragging.value = false
}

// Toggle zoom: if already zoomed → reset; else zoom to 2x anchored at
// the click/tap point. Position calc: place the clicked point at the
// container center so the user gets a satisfying "zoom into where I
// looked" feel rather than zoom-from-center.
const handleDoubleClickAt = (clientX: number, clientY: number) => {
  const rect = containerRef.value?.getBoundingClientRect()
  if (!rect) return
  if (scale.value > MIN_SCALE) {
    resetTransform()
    return
  }
  const px = clientX - rect.left
  const py = clientY - rect.top
  scale.value = 2
  const constrained = constrainPosition(
    px - rect.width / 2,
    py - rect.height / 2
  )
  position.x = constrained.x
  position.y = constrained.y
}

// Mouse double-click. Browser fires this for actual mouse devices; the
// touchscreen path goes through onTouchEnd's manual double-tap detect.
const handleDoubleClick = (e: MouseEvent) =>
  handleDoubleClickAt(e.clientX, e.clientY)

const handleTouchStart = (e: TouchEvent) => {
  if (e.touches.length >= 2) {
    // Enter pinch mode. Cancel any single-touch drag that may have
    // started a frame earlier — once a second finger lands, we treat
    // the whole gesture as pinch until ALL fingers leave.
    isDragging.value = false
    isPinching.value = true
    const [t1, t2] = [e.touches[0]!, e.touches[1]!]
    lastTouchDistance.value = Math.hypot(
      t2.clientX - t1.clientX,
      t2.clientY - t1.clientY
    )
    lastTouch.x = (t1.clientX + t2.clientX) / 2
    lastTouch.y = (t1.clientY + t2.clientY) / 2
  } else if (e.touches.length === 1 && !isPinching.value) {
    startDrag(e)
  }
}

const handleTouchMove = (e: TouchEvent) => {
  if (e.touches.length >= 2 && isPinching.value) {
    // Pinch zoom anchored at the finger midpoint. Standard formula
    // (mirrors handleWheel): newPos = anchor - (anchor - oldPos) *
    // scaleChange. Then add the midpoint translation since last
    // frame so a pan-while-pinching feels natural too.
    //
    // distance ratio (multiplicative) — better than additive `* 0.01`
    // which was scale-unaware and felt jumpy at low zoom levels.
    const [t1, t2] = [e.touches[0]!, e.touches[1]!]
    const currentDistance = Math.hypot(
      t2.clientX - t1.clientX,
      t2.clientY - t1.clientY
    )
    const centerX = (t1.clientX + t2.clientX) / 2
    const centerY = (t1.clientY + t2.clientY) / 2
    const distRatio = currentDistance / (lastTouchDistance.value || 1)
    const newScale = Math.max(
      MIN_SCALE,
      Math.min(MAX_SCALE, scale.value * distRatio)
    )

    const rect = containerRef.value?.getBoundingClientRect()
    if (rect) {
      // Anchor expressed relative to container center (= the same
      // coordinate space `position` lives in — transform-origin is
      // the image's own center).
      const ax = centerX - rect.left - rect.width / 2
      const ay = centerY - rect.top - rect.height / 2
      const sc = newScale / scale.value
      const zoomedX = ax - (ax - position.x) * sc
      const zoomedY = ay - (ay - position.y) * sc
      const dx = centerX - lastTouch.x
      const dy = centerY - lastTouch.y
      const constrained = constrainPosition(zoomedX + dx, zoomedY + dy)
      position.x = constrained.x
      position.y = constrained.y
    }
    scale.value = newScale
    lastTouchDistance.value = currentDistance
    lastTouch.x = centerX
    lastTouch.y = centerY
  } else if (e.touches.length === 1 && isDragging.value) {
    onDrag(e)
  }
}

// Touch end handles three flows:
//   1. Pinch end with 1 finger remaining → switch to drag mode, reset
//      drag baseline to that finger so the pan doesn't snap.
//   2. Pinch end with 0 fingers → exit pinch, no swipe detect (swipe
//      would have wrong baseline from before the pinch).
//   3. Normal single-touch end → delegate to stopDrag, then check for
//      double-tap (since @dblclick is mobile-unreliable).
const onTouchEnd = (e: TouchEvent) => {
  if (isPinching.value) {
    if (e.touches.length >= 2) return // still pinching (3+ → 2 valid)
    isPinching.value = false
    if (e.touches.length === 1) {
      // One finger left → seamlessly become a drag. Re-initialize the
      // drag baseline so the very next touchmove computes deltas
      // relative to the current finger position, not the pre-pinch
      // single-touch state (which would cause a visible jump).
      const point = e.touches[0]!
      isDragging.value = true
      dragStartTime.value = Date.now()
      dragStart.x = point.clientX - position.x
      dragStart.y = point.clientY - position.y
      initialDragPosition.x = point.clientX
      initialDragPosition.y = point.clientY
    }
    return // skip swipe + double-tap detect on pinch end
  }

  stopDrag(e)

  // Double-tap detect — fires only when ALL fingers up (touches.length
  // === 0) AND the gesture wasn't a swipe (initialDragPosition delta
  // small enough). Bail out otherwise.
  if (e.touches.length !== 0) return
  const t = e.changedTouches[0]
  if (!t) return
  const movedDuringTap =
    Math.abs(t.clientX - initialDragPosition.x) > DOUBLE_TAP_PX ||
    Math.abs(t.clientY - initialDragPosition.y) > DOUBLE_TAP_PX
  if (movedDuringTap) {
    lastTapTime = 0 // movement disqualifies this from being half of a double-tap
    return
  }
  const now = Date.now()
  if (
    now - lastTapTime < DOUBLE_TAP_MS &&
    Math.abs(t.clientX - lastTapX) < DOUBLE_TAP_PX &&
    Math.abs(t.clientY - lastTapY) < DOUBLE_TAP_PX
  ) {
    handleDoubleClickAt(t.clientX, t.clientY)
    lastTapTime = 0 // reset so triple-tap doesn't count as another double
  } else {
    lastTapTime = now
    lastTapX = t.clientX
    lastTapY = t.clientY
  }
}

// Image download with CORS-aware fallback.
//
// Happy path: same-origin, OR cross-origin where the server sends
// Access-Control-Allow-Origin: <current origin> + has a CORP/COEP setup
// that lets us read the bytes. We fetch → blob → object URL → <a download>,
// so the browser saves to disk with a clean filename and no extra prompt.
//
// Fallback: cross-origin CDN that doesn't send CORS headers (e.g. Cloudflare
// R2 without explicit bucket-level CORS rules). fetch() throws TypeError
// "Failed to fetch" — caught here and recovered by opening the raw URL in
// a new tab. The browser's native context menu lets the user save from
// there. UX is one click worse than the happy path, but avoids the
// previous silent failure that left users staring at console errors.
const downloadImage = async () => {
  const url = currentImage.value
  if (!url) return
  const filename = url.split('/').pop() || 'image'

  try {
    const response = await fetch(url, { mode: 'cors' })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    const blob = await response.blob()
    const objectUrl = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    window.URL.revokeObjectURL(objectUrl)
  } catch (error) {
    // CORS / network failure → open in new tab as a manual-save fallback.
    // Log at warn (not error) since this is an expected, recovered path.
    console.warn(
      '[KunLightbox] direct download blocked (likely CORS), opening in new tab',
      error
    )
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

// Jump to a specific index — used by the thumbnail strip and the
// mobile pagination dots. Direction-aware so the slide transition
// goes the natural way (clicking a thumbnail to the right of current
// slides leftward, etc.). No-op when clicking the active item.
const goToIndex = (i: number) => {
  if (i === currentIndex.value || i < 0 || i >= props.images.length) return
  slideDir.value = i > currentIndex.value ? 'slide-next' : 'slide-prev'
  resetTransform()
  currentIndex.value = i
}

// Live zoom percentage shown in the toolbar between zoom−/zoom+. Rounded
// to whole numbers — pinch/wheel can produce fractional scales (e.g.
// 1.07) which would jitter visually if displayed at full precision.
const zoomPercent = computed(() => Math.round(scale.value * 100))

// Keyboard nav. Scoped to the dialog's own keydown so the listener
// doesn't fire across the whole document when closed. ESC is handled
// natively by <dialog> + onDialogClose, we only own ←/→.
const onDialogKeydown = (e: KeyboardEvent) => {
  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    prev()
  } else if (e.key === 'ArrowRight') {
    e.preventDefault()
    next()
  }
}

onUnmounted(() => {
  if (locked) unlock()
})
</script>

<template>
  <dialog
    ref="dialogRef"
    aria-label="图片查看器"
    class="kun-lightbox-dialog bg-transparent text-foreground p-0 m-0 max-w-none max-h-none w-screen h-screen overflow-hidden backdrop:bg-default-800/80"
    @close="onDialogClose"
    @click="onDialogClick"
    @keydown="onDialogKeydown"
  >
    <div
      v-if="isOpen"
      class="relative flex h-full w-full items-center justify-center"
    >
      <!-- Image surface. Wraps the <img> in a positioned Transition so
           prev/next swap-and-slide. The inner div is the transition
           target; the img inside still owns its own zoom/pan/rotate
           transform (those compose, not conflict). -->
      <div
        ref="containerRef"
        class="kun-lightbox-stage relative flex h-full w-full touch-none items-center justify-center overflow-hidden"
        @wheel.prevent="handleWheel"
        @mousedown="startDrag"
        @mousemove="onDrag"
        @mouseup="stopDrag"
        @mouseleave="stopDrag"
        @touchstart.passive="handleTouchStart"
        @touchmove.passive="handleTouchMove"
        @touchend="onTouchEnd"
        @touchcancel="onTouchEnd"
        @dblclick="handleDoubleClick"
      >
        <!-- No mode = both old and new slide simultaneously (carousel
             style). The absolute-positioned slide wraps stack on top
             of each other during the ~0.35s transition window. -->
        <Transition :name="slideDir">
          <div
            :key="currentIndex"
            class="kun-lightbox-slide absolute inset-0 flex items-center justify-center"
          >
            <img
              ref="imageRef"
              :src="currentImage"
              :alt="currentAlt"
              class="kun-lightbox-image max-h-full max-w-full will-change-transform"
              :style="transformStyle"
              draggable="false"
              @click.stop
            />
          </div>
        </Transition>
      </div>

      <!-- ─────────────────────────────────────────────────────────
           Floating chrome layers — 5 zones, each absolute-positioned.
           All share the same KunUI surface treatment (semi-transparent
           dark fill + backdrop blur + subtle border) so they read as
           one coherent system without sitting in a single bounding box.
           ───────────────────────────────────────────────────────── -->

      <!-- Top-left: page counter. Hidden in single-image mode (the
           "1 / 1" would just be noise). -->
      <div
        v-if="images.length > 1"
        class="absolute top-4 left-4 z-50 rounded-lg border border-white/10 bg-black/70 px-3 py-1.5 text-sm font-medium text-white shadow-lg backdrop-blur-md"
        aria-live="polite"
      >
        {{ currentIndex + 1 }} / {{ images.length }}
      </div>

      <!-- Top-right: close. Lives apart from the toolbar by convention
           (universal "X to dismiss" position) — fastest path to escape
           regardless of where the pointer was when the user changed
           their mind. -->
      <KunButton
        :is-icon-only="true"
        color="default"
        variant="light"
        size="lg"
        rounded="lg"
        aria-label="关闭"
        class-name="absolute top-4 right-4 z-50 bg-black/70 backdrop-blur-md border border-white/10 shadow-lg"
        @click.stop="emit('update:isOpen', false)"
      >
        <KunIcon name="lucide:x" class="text-white" />
      </KunButton>

      <!-- Edge arrows: PC only (md+). On mobile, touch-swipe in
           stopDrag is the primary nav, and dots below give one-tap
           jumps. Vertically centered via top-1/2 + -translate-y-1/2. -->
      <template v-if="images.length > 1">
        <KunButton
          :is-icon-only="true"
          color="default"
          variant="light"
          size="xl"
          rounded="lg"
          aria-label="上一张"
          class-name="absolute left-4 top-1/2 z-50 hidden -translate-y-1/2 bg-black/70 backdrop-blur-md border border-white/10 shadow-lg md:flex"
          @click.stop="prev"
        >
          <KunIcon name="lucide:chevron-left" class="text-white" />
        </KunButton>
        <KunButton
          :is-icon-only="true"
          color="default"
          variant="light"
          size="xl"
          rounded="lg"
          aria-label="下一张"
          class-name="absolute right-4 top-1/2 z-50 hidden -translate-y-1/2 bg-black/70 backdrop-blur-md border border-white/10 shadow-lg md:flex"
          @click.stop="next"
        >
          <KunIcon name="lucide:chevron-right" class="text-white" />
        </KunButton>
      </template>

      <!-- Bottom area: thumbnails (PC) + toolbar (both). Stacked in a
           flex-col with center-aligned children so they read as one
           grouped panel even though they're separate pills. The toolbar
           is mandatory; the thumbnail strip and mobile dots are
           conditional. -->
      <div
        class="pointer-events-none absolute right-0 bottom-6 left-0 z-50 flex flex-col items-center gap-2"
      >
        <!-- Mobile: pagination dots. Compact, tap-to-jump. Hidden on
             md+ where the thumbnail strip provides richer navigation. -->
        <div
          v-if="images.length > 1"
          class="pointer-events-auto flex items-center gap-2 rounded-full border border-white/10 bg-black/70 px-3 py-2 shadow-lg backdrop-blur-md md:hidden"
        >
          <button
            v-for="(_, i) in images"
            :key="`dot-${i}`"
            type="button"
            :aria-label="`跳转到第 ${i + 1} 张`"
            :aria-current="i === currentIndex"
            class="size-2 rounded-full transition-colors"
            :class="i === currentIndex ? 'bg-primary-500' : 'bg-default-400/60 hover:bg-default-300'"
            @click.stop="goToIndex(i)"
          />
        </div>

        <!-- PC: thumbnail strip. overflow-x-auto so long galleries
             horizontally scroll within the strip rather than push the
             toolbar off-screen. The active thumbnail gets a ring
             highlight to anchor the user's place. -->
        <div
          v-if="images.length > 1"
          class="pointer-events-auto hidden max-w-[80vw] items-center gap-1.5 overflow-x-auto rounded-xl border border-white/10 bg-black/70 p-2 shadow-lg backdrop-blur-md md:flex"
        >
          <button
            v-for="(img, i) in images"
            :key="`thumb-${i}`"
            type="button"
            :aria-label="`跳转到第 ${i + 1} 张`"
            :aria-current="i === currentIndex"
            class="shrink-0 overflow-hidden rounded-md border-2 transition-all"
            :class="
              i === currentIndex
                ? 'border-primary-500 opacity-100'
                : 'border-transparent opacity-60 hover:opacity-100'
            "
            @click.stop="goToIndex(i)"
          >
            <img
              :src="img.src"
              :alt="img.alt ?? ''"
              class="size-14 object-cover"
              draggable="false"
            />
          </button>
        </div>

        <!-- Toolbar: zoom-out / percent / zoom-in | rotate-left /
             rotate-right | reset | download.
             Buttons bumped from md to lg per spec. Sections separated
             by hairline dividers — same pattern as before, just no
             nav/close (those moved to edges/corners). -->
        <div
          class="kun-lightbox-toolbar pointer-events-auto flex items-center gap-1 rounded-full border border-white/10 bg-black/70 px-2 py-1.5 shadow-lg backdrop-blur-md"
          @click.stop
        >
          <KunButton
            :is-icon-only="true"
            color="default"
            variant="light"
            size="lg"
            rounded="full"
            aria-label="缩小"
            @click="zoomOut"
          >
            <KunIcon name="lucide:zoom-out" class="text-white" />
          </KunButton>
          <span
            class="min-w-[3.5rem] text-center text-sm font-medium tabular-nums text-white"
            aria-live="polite"
          >
            {{ zoomPercent }}%
          </span>
          <KunButton
            :is-icon-only="true"
            color="default"
            variant="light"
            size="lg"
            rounded="full"
            aria-label="放大"
            @click="zoomIn"
          >
            <KunIcon name="lucide:zoom-in" class="text-white" />
          </KunButton>
          <span class="mx-1 h-5 w-px bg-default-200/30" aria-hidden="true" />
          <KunButton
            :is-icon-only="true"
            color="default"
            variant="light"
            size="lg"
            rounded="full"
            aria-label="向左旋转 90°"
            @click="rotateLeft"
          >
            <KunIcon name="lucide:rotate-ccw" class="text-white" />
          </KunButton>
          <KunButton
            :is-icon-only="true"
            color="default"
            variant="light"
            size="lg"
            rounded="full"
            aria-label="向右旋转 90°"
            @click="rotateRight"
          >
            <KunIcon name="lucide:rotate-cw" class="text-white" />
          </KunButton>
          <span class="mx-1 h-5 w-px bg-default-200/30" aria-hidden="true" />
          <KunButton
            :is-icon-only="true"
            color="default"
            variant="light"
            size="lg"
            rounded="full"
            aria-label="重置缩放/旋转/位置"
            @click="resetTransform"
          >
            <KunIcon name="lucide:refresh-ccw" class="text-white" />
          </KunButton>
          <KunButton
            :is-icon-only="true"
            color="default"
            variant="light"
            size="lg"
            rounded="full"
            aria-label="下载"
            @click="downloadImage"
          >
            <KunIcon name="lucide:download" class="text-white" />
          </KunButton>
        </div>
      </div>
    </div>
  </dialog>
</template>

<style scoped>
/* Opt-in morph-from-thumbnail: consumers add the same view-transition-name
 * to their trigger thumbnail and the API auto-animates between them.
 * Default fade still applies when no matching name on the page. */
.kun-lightbox-image {
  view-transition-name: kun-lightbox-image;
}

/* ::backdrop is the native dialog's overlay. Tailwind utilities on the
 * dialog element via `backdrop:bg-…` don't always apply in scoped CSS
 * pipelines, so we also restate it here as a defence-in-depth. */
.kun-lightbox-dialog::backdrop {
  background: rgba(31, 41, 55, 0.8);
  backdrop-filter: blur(2px);
}

/* Carousel-style image swap. Vue's <Transition> with no `mode` puts
 * leave and enter elements in the DOM simultaneously; both slides are
 * absolute-positioned so they overlap and we see the old image glide
 * out as the new image glides in, cinemascope-style. cubic-bezier
 * (0.4, 0, 0.2, 1) is Material's "standard" easing — quick start,
 * gentle settle, no overshoot. */
.slide-next-enter-active,
.slide-next-leave-active,
.slide-prev-enter-active,
.slide-prev-leave-active {
  transition:
    transform 0.35s cubic-bezier(0.4, 0, 0.2, 1),
    opacity 0.35s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Forward (next): outgoing slide flies LEFT, incoming arrives FROM RIGHT.
 * Matches the user's mental model of "the next image is to the right". */
.slide-next-enter-from {
  transform: translateX(100%);
  opacity: 0;
}
.slide-next-leave-to {
  transform: translateX(-100%);
  opacity: 0;
}

/* Backward (prev): mirror of slide-next — outgoing flies RIGHT, incoming
 * arrives FROM LEFT. */
.slide-prev-enter-from {
  transform: translateX(-100%);
  opacity: 0;
}
.slide-prev-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
</style>
