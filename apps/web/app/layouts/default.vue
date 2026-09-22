<template>
  <div class="bg-radial relative flex min-h-screen flex-col items-center">
    <KunTopBarNSFWBanner />

    <KunTopBar />

    <div class="mt-4 w-full grow px-3">
      <div
        class="mx-auto flex min-h-[calc(100dvh-24rem)] w-full max-w-7xl"
      >
        <slot />
      </div>
    </div>

    <KunBackToTop />
    <KunFooter />

    <AuthLoginModal />

    <LogoutModal />
  </div>
</template>
