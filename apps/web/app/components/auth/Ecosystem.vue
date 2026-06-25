<script setup lang="ts">
// The OAuth ecosystem strip on the login modal: "也可以用鲲 Galgame 账号登录以下
// 网站" + a row of the sibling sites' logos. Pure presentation — it never touches
// the auth flow.
//
// Data path: we fetch moyu's OWN same-origin GET /auth/oauth/ecosystem, which
// proxies + TTL-caches the OAuth provider's public app directory server-to-
// server. Fetching same-origin sidesteps CORS — moyu's TLD is not in the
// provider's allow-list, so a direct browser fetch would be blocked.
//
// Renders NOTHING when the list is empty (cold-start upstream miss, or the
// provider has no opt-in sites) so the login modal degrades cleanly.
const api = useApi()

const apps = ref<EcosystemApp[]>([])

// Client-only, best-effort. The strip is decorative, so a failure is swallowed
// to an empty list rather than surfaced. Fetched on mount (the modal mounts once
// globally); the backend's TTL cache keeps this near-instant.
onMounted(async () => {
  const res = await api.get<{ apps: EcosystemApp[] }>('/auth/oauth/ecosystem')
  if (res.code === 0 && Array.isArray(res.data?.apps)) {
    apps.value = res.data.apps
  }
})

// Initial-letter fallback when an app has no logo_url (or the image 404s).
const initialOf = (name: string) => (name.trim()[0] ?? '?').toUpperCase()

// Track which logos failed to load so we can swap in the initial fallback.
const failed = ref<Record<string, boolean>>({})
const onLogoError = (domain: string) => {
  failed.value[domain] = true
}
</script>

<template>
  <div v-if="apps.length" class="flex w-full flex-col items-center gap-3">
    <KunDivider class="w-full" />

    <p class="text-default-400 text-center text-xs">
      也可以用鲲 Galgame 账号登录以下网站
    </p>

    <div class="flex flex-wrap items-center justify-center gap-3">
      <KunTooltip
        v-for="app in apps"
        :key="app.site_domain"
        :text="app.tagline || app.name"
      >
        <a
          :href="`https://${app.site_domain}`"
          target="_blank"
          rel="noopener noreferrer"
          class="border-default-200 hover:border-primary flex items-center gap-2 rounded-full border px-3 py-1.5 transition-colors"
        >
          <img
            v-if="app.logo_url && !failed[app.site_domain]"
            :src="app.logo_url"
            :alt="app.name"
            class="size-5 rounded-full object-cover"
            loading="lazy"
            @error="onLogoError(app.site_domain)"
          />
          <span
            v-else
            class="bg-default-100 text-default-500 flex size-5 items-center justify-center rounded-full text-xs font-bold"
          >
            {{ initialOf(app.name) }}
          </span>

          <span class="text-foreground text-xs">{{ app.name }}</span>
        </a>
      </KunTooltip>
    </div>
  </div>
</template>
