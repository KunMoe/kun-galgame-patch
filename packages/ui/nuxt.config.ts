import { fileURLToPath } from 'node:url'

const currentDir = fileURLToPath(new URL('.', import.meta.url))

export default defineNuxtConfig({
  modules: ['@nuxt/icon', '@nuxt/image'],

  // Default provider stays IPX — most KunUI consumers route runtime-
  // resized galgame banners / user avatars through it. The `none`
  // provider is registered here at the layer level so any KunImage
  // call site downstream can opt out via `provider="none"` for
  // pre-optimized static assets (author-time AVIF banners etc.)
  // without each consuming app having to add this to its own
  // nuxt.config. The IPX round-trip on already-optimized images is
  // pure latency overhead — first hit spins up sharp on the server
  // and the default 300s FS cache means cold-loads bunch up.
  image: {
    providers: {
      none: { name: 'none', provider: '@nuxt/image/runtime/providers/none' }
    }
  },

  css: [`${currentDir}/app/styles/tailwindcss.css`],
})
