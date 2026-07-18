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
  // @kungal/editor-nuxt is the KunEditor Nuxt layer — it auto-imports <KunEditor>
  // (the shared Milkdown editor) the same way @kungal/ui-nuxt does its components,
  // replacing the deleted in-repo components/kun/milkdown/ port. Host policy
  // (upload / mention / sticker / notify) is injected per call site via
  // useKunEditorAdapters(); see app/composables/useKunEditorAdapters.ts.
  extends: ['@kungal/ui-nuxt', '@kungal/editor-nuxt'],

  devtools: { enabled: false },

  modules: [
    '@nuxt/image',
    '@nuxt/eslint',
    '@nuxtjs/color-mode',
    '@nuxtjs/sitemap',
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

  site: {
    // Drives both nuxt-schema-org and @nuxtjs/sitemap (shared nuxt-site-config).
    // Literal fallback because moyu-web is built GENERIC (the Docker build passes
    // no site build-arg), so an unset env at build → an empty site.url would make
    // sitemap <loc>s non-absolute. Prod runs on this canonical host; override at
    // runtime with NUXT_PUBLIC_SITE_URL if it ever moves.
    url: process.env.NUXT_PUBLIC_SITE_URL || 'https://www.moyu.moe'
  },

  sitemap: {
    // Keep private / auth-gated / editor / non-content routes OUT of the
    // auto-discovered static pages. Routes with params (/patch/[id],
    // /user/[id], /resource/[id], /doc/[...slug]) are never auto-included —
    // those come from the dynamic source below — so only param-free pages
    // need listing here.
    exclude: [
      '/admin',
      '/admin/**',
      '/auth/**',
      '/edit/**',
      '/me',
      '/me/**',
      '/message',
      '/message/**',
      '/settings',
      '/settings/**',
      '/user'
    ],
    // Emit a /sitemap_index.xml of ≤1000-URL chunks (Google caps a single file
    // at 50k/50MB; smaller chunks cache + debug better and give headroom as
    // content grows). In 7.x, `chunks` is only honoured inside the multi-sitemap
    // `sitemaps` map — not on the default single sitemap.
    defaultSitemapsChunkSize: 1000,
    sitemaps: {
      moyu: {
        // Static, param-free pages auto-discovered from the route table.
        includeAppSources: true,
        // Dynamic content URLs (patches/resources/docs) from this runtime
        // endpoint, which enumerates the Go API with the SFW filter (BE default
        // when no content_limit is sent). Runs at request time (cached) — never
        // at build, which has no Go-API access.
        sources: ['/api/__sitemap__/urls'],
        chunks: true
      }
    },
    // Cache the rendered sitemap; the source endpoint is cached independently too.
    cacheMaxAgeSeconds: 60 * 60 * 6,
    defaults: { changefreq: 'daily', priority: 0.7 }
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
    // These cookies (userStore / settingStore) are JS-readable DISPLAY/PREF
    // mirrors — identity for first-paint (name/avatar/roles) + the NSFW
    // content_limit preference. The real auth boundary is the httpOnly
    // `moyu_session` cookie, which is SameSite=Lax (see api auth.go).
    //
    // They MUST be Lax, not Strict: Strict cookies are withheld on a cross-site
    // top-level navigation (clicking a moyu link FROM another site), so the SSR
    // pass reads user.id=0 + kunNsfwEnable=sfw and bakes content_limit=sfw into
    // the first wiki call — which 404s a logged-in user's NSFW detail page
    // ("资源不存在") until a same-site refresh re-sends the cookie. Lax sends the
    // cookie on top-level GET navigations (fixing that) while still blocking it
    // on cross-site POST/iframe/subrequests, so there's no CSRF exposure — and
    // it matches the session cookie these mirror.
    cookieOptions: {
      maxAge: 60 * 60 * 24 * 7,
      sameSite: 'lax'
    }
  },

  colorMode: {
    preference: 'system',
    fallback: 'light',
    globalName: '__KUNGALGAME_COLOR_MODE__',
    componentName: 'ColorScheme',
    classPrefix: 'kun-',
    classSuffix: '-mode',
    storageKey: 'kungalgame-color-mode',
    // Cookie (not the default localStorage) so the SSR plugin can read the saved
    // preference from the request and render colorMode.preference correctly —
    // without this, any markup that reads the preference during SSR (the
    // /settings/system theme picker) hydration-mismatches because the server
    // falls back to 'system' while the client reads the real value. Matches
    // moyu's other cookie-backed prefs (settingStore) for the same SSR reason.
    storage: 'cookie'
  },

  vite: {
    plugins: [tailwindcss()]
    // NB: the Vite dev optimizeDeps for the editor/Milkdown chain (the
    // micromark→`debug` CJS interop fix) is now owned by the @kungal/editor-nuxt
    // layer (>=0.13.0) and merged in automatically — moyu no longer configures it.
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
      // image_service CDN base (== backend KUN_IMAGE_CDN_BASE / config
      // moyu-moe.ts domain.imageBed — all three MUST agree). Used by the
      // /image/:hash 302 route to resolve domain-agnostic content tokens to
      // `{base}/<aa>/<bb>/<hash>.webp`. Overridable at runtime so a domain
      // change is "改一处配置" (image_service 契约 04).
      imageBed:
        process.env.NUXT_PUBLIC_IMAGE_BED || 'https://image.kungal.iloveren.link'
    }
  }
})
