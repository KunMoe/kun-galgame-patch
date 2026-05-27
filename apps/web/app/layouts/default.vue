<template>
  <div class="bg-radial relative flex min-h-screen flex-col items-center">
    <!-- Global NSFW mode indicator. Sits above the (sticky) top bar so it
         scrolls away naturally; doesn't compete with the nav for the top
         edge while the user is browsing. Always rendered — danger colour
         when SFW (content hidden), success colour when "show all" is on. -->
    <KunTopBarNSFWBanner />

    <KunTopBar />

    <!-- Padding scheme mirrors the top bar: `px-3` sits on the OUTER full-
         width wrapper (giving screen-edge breathing room at all viewports),
         and the inner `max-w-7xl` slot has no padding so page content is
         flush with the centered 1280px edges — i.e. left/right aligned to
         where the top-bar content also starts. Putting `px-3` inside
         `max-w-7xl` would inset pages 12px further than the top bar at
         >= xl widths. -->
    <div class="mt-4 w-full grow px-3">
      <div
        class="mx-auto flex min-h-[calc(100dvh-24rem)] w-full max-w-7xl"
      >
        <slot />
      </div>
    </div>

    <KunBackToTop />
    <KunFooter />
  </div>
</template>
