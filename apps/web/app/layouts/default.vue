<template>
  <div class="bg-radial relative flex min-h-screen flex-col items-center">
    <KunTopBar />

    <div
      class="mt-4 flex min-h-[calc(100dvh-24rem)] w-full max-w-7xl grow px-3"
    >
      <slot />
    </div>

    <KunBackToTop />
    <KunFooter />
  </div>
</template>
