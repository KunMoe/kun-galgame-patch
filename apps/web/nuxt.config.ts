import tailwindcss from '@tailwindcss/vite'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',

  // KunUI is consumed as the published npm Nuxt layer (@kungal/ui-nuxt). It
  // auto-imports every <KunButton> / <KunChip> / ... component + composables
  // from @kungal/ui-vue and injects NuxtLink / @nuxt/icon (fallback) / @nuxt/
  // image. Unlike the old in-repo @kun/ui layer, it ships NO Tailwind entry —
  // moyu owns that in styles/index.css (tokens + @source). Design tokens come
  // from @kungal/ui-tokens; icons are registered in app/plugins/kun-icons.ts.
  extends: ['@kungal/ui-nuxt'],

  devtools: { enabled: false },

  modules: [
    '@nuxt/image',
    '@nuxt/eslint',
    '@nuxtjs/color-mode',
    '@pinia/nuxt',
    'pinia-plugin-persistedstate/nuxt',
    'nuxt-schema-org',
    'nuxt-umami'
  ],

  // Default provider stays IPX (galgame banners / user avatars need on-the-
  // -fly resize / re-encode). `none` is registered as a NAMED provider so
  // pre-optimized static assets (e.g. about post banners — already AVIF at
  // author time) can opt out per-image via `<NuxtImg provider="none">` and
  // skip the IPX → sharp roundtrip + filesystem cache miss latency.
  image: {
    providers: {
      none: { name: 'none', provider: '@nuxt/image/runtime/providers/none' }
    }
  },


  devServer: {
    host: '127.0.0.1',
    port: 6969
  },

  // Frontend
  css: ['~/styles/index.css'],

  imports: {
    dirs: ['shared/utils/**']
  },

  pinia: {
    storesDirs: ['./stores/**']
  },

  piniaPluginPersistedstate: {
    cookieOptions: {
      maxAge: 60 * 60 * 24 * 7,
      sameSite: 'strict'
    }
  },

  colorMode: {
    preference: 'system',
    fallback: 'light',
    globalName: '__KUNGALGAME_COLOR_MODE__',
    componentName: 'ColorScheme',
    classPrefix: 'kun-',
    classSuffix: '-mode',
    storageKey: 'kungalgame-color-mode'
  },

  vite: {
    plugins: [tailwindcss()]
  },

  umami: {
    // Public website-id (it ships in the client tracker tag — not a secret). The
    // `|| '<id>'` literal is the actual config: moyu-web is built GENERIC in CI
    // (no build-args) and .env is gitignored, so at BUILD time the env var is
    // undefined → nuxt-umami would bake an undefined id and every /api/send would
    // 400. nuxt-umami reads `id` at build (a module option), not at runtime, so
    // the default must be a literal here. (Same fix kungal-forum landed.)
    id:
      process.env.KUN_VISUAL_NOVEL_PATCH_UMAMI_ID ||
      'da1440d0-60f7-4d5d-9a91-b4ccfb5d4b37',
    host: 'https://umami.kungal.org/',
    autoTrack: true,
    // One baked id runs in every deployment (incl. local dev) — keep dev traffic
    // out of prod stats.
    ignoreLocalhost: true
  },

  runtimeConfig: {
    // SSR runs inside the docker container, where the Go API is reachable by
    // its compose service name (api:5214) — NOT by the browser's host-port URL
    // (localhost:15010 is the container's own loopback). The browser can't
    // resolve `api`, so it keeps using public.apiBase. Set
    // NUXT_API_BASE_SSR=http://moyu-api:5214/api/v1 in docker; leave empty for local
    // air dev (the dual-base reader falls back to public.apiBase).
    apiBaseSsr: process.env.NUXT_API_BASE_SSR || '',
    public: {
      // 本项目 Go Fiber API（不是 鲲 Galgame OAuth）。Go 端口从 apps/api/.env 的 KUN_SERVER_PORT 读，dev 默认 5214。
      apiBase:
        process.env.KUN_VISUAL_NOVEL_NUXT_PUBLIC_API_BASE ||
        'http://127.0.0.1:5214/api/v1',
      // 鲲 Galgame OAuth — 拆成两个 URL：
      // - oauthServerUrl：API base，给 fetch / 拿 token / 调 /users/batch 用（dev :9277）
      // - oauthWebUrl：用户面前端，给 window.location.href 跳转 /oauth/authorize / /register / /forgot / /profile 用（dev :9420）
      // prod 同域 `oauth.kungal.com`，但开发环境是两个端口，所以不能再从 API URL 推 origin。
      oauthServerUrl:
        process.env.NUXT_PUBLIC_KUN_OAUTH_SERVER_URL ||
        'http://127.0.0.1:9277/api/v1',
      oauthWebUrl:
        process.env.NUXT_PUBLIC_KUN_OAUTH_WEB_URL || 'http://127.0.0.1:9420',
      oauthClientId: process.env.NUXT_PUBLIC_KUN_OAUTH_CLIENT_ID || '',
      oauthRedirectUri: process.env.NUXT_PUBLIC_KUN_OAUTH_REDIRECT_URI || '',
      // 鲲 Galgame Wiki 的公开 origin。用于：(1) 跳转 wiki 的 galgame 详情页
      // `${wikiOrigin}/galgame/:id`；(2) /edit/draft 直连 wiki 读 API
      // `${wikiOrigin}/api/galgame/:id`（wiki 对读端点开了 CORS）。
      // 注意是 wiki.kungal.com，不是 galgame.kungal.com。运行时可由
      // NUXT_PUBLIC_WIKI_ORIGIN 覆盖（Nitro 自动映射 camelCase→SCREAMING_SNAKE）。
      wikiOrigin:
        process.env.NUXT_PUBLIC_WIKI_ORIGIN || 'https://wiki.kungal.com'
    }
  }
})
