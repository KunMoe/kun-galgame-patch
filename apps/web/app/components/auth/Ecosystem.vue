<script setup lang="ts">
const api = useApi()

const apps = ref<EcosystemApp[]>([])

const expanded = ref(false)

const utmSource = ref('')

onMounted(async () => {
  utmSource.value = window.location.hostname
  const res = await api.get<{ apps: EcosystemApp[] }>('/auth/oauth/ecosystem')
  if (res.code === 0 && Array.isArray(res.data?.apps)) {
    apps.value = res.data.apps
  }
})

const sortedApps = computed(() =>
  [...apps.value].sort(
    (a, b) => Number(b.auto_consent ?? false) - Number(a.auto_consent ?? false)
  )
)

const hrefFor = (app: EcosystemApp) => {
  try {
    const url = new URL(`https://${app.site_domain}`)
    if (utmSource.value) url.searchParams.set('utm_source', utmSource.value)
    return url.toString()
  } catch {
    return `https://${app.site_domain}`
  }
}

const initialOf = (name: string) => (name.trim()[0] ?? '?').toUpperCase()

const failed = ref<Record<string, boolean>>({})
const onLogoError = (domain: string) => {
  failed.value[domain] = true
}

const scrollEl = ref<HTMLElement | null>(null)
const atTop = ref(true)
const atBottom = ref(true)

const updateShadow = () => {
  const el = scrollEl.value
  if (!el) return
  atTop.value = el.scrollTop <= 1
  atBottom.value = el.scrollHeight - el.clientHeight - el.scrollTop <= 1
}

const scrollShadow = computed(() => {
  const parts: string[] = []
  if (!atTop.value) parts.push('inset 0 7px 6px -7px rgb(0 0 0 / 0.18)')
  if (!atBottom.value) parts.push('inset 0 -7px 6px -7px rgb(0 0 0 / 0.18)')
  return parts.length ? { boxShadow: parts.join(', ') } : {}
})

watch(expanded, (open) => {
  if (open) nextTick(updateShadow)
})
</script>

<template>
  <div v-if="apps.length" class="flex w-full flex-col items-center gap-3">
    <KunDivider class="w-full" />

    <p class="text-default-500 text-center text-xs">
      拥有<span class="text-primary font-semibold">NextMoe·未萌</span>账号，一键登录以下全部 ACG 网站
    </p>

    <div class="flex flex-wrap items-center justify-center gap-2">
      <KunTooltip
        v-for="app in sortedApps"
        :key="app.site_domain"
        :text="app.tagline || app.name"
      >
        <a
          :href="hrefFor(app)"
          target="_blank"
          rel="noopener noreferrer"
          class="border-default-200 bg-content1 hover:border-primary flex size-7 items-center justify-center overflow-hidden rounded-full border transition-colors"
        >
          <img
            v-if="app.logo_url && !failed[app.site_domain]"
            :src="app.logo_url"
            :alt="app.name"
            class="size-full object-cover"
            loading="lazy"
            @error="onLogoError(app.site_domain)"
          />
          <span v-else class="text-default-500 text-xs font-bold">
            {{ initialOf(app.name) }}
          </span>
        </a>
      </KunTooltip>

      <button
        type="button"
        :aria-expanded="expanded"
        :aria-label="expanded ? '收起网站列表' : '展开网站列表'"
        class="text-default-400 hover:text-primary hover:bg-default-100 flex size-7 items-center justify-center rounded-full transition-colors"
        @click="expanded = !expanded"
      >
        <KunIcon
          name="lucide:chevron-down"
          class="size-4 transition-transform duration-300"
          :class="expanded ? 'rotate-180' : ''"
        />
      </button>
    </div>

    <div
      class="grid w-full transition-all duration-300 ease-out"
      :class="expanded ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0'"
    >
      <div class="overflow-hidden">
        <div
          ref="scrollEl"
          class="scrollbar-hide max-h-[100px] overflow-y-auto pt-1"
          :style="scrollShadow"
          @scroll="updateShadow"
        >
          <div class="flex flex-col gap-0.5">
            <a
              v-for="app in sortedApps"
              :key="app.site_domain"
              :href="hrefFor(app)"
              target="_blank"
              rel="noopener noreferrer"
              class="hover:bg-default-100 flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-colors"
            >
              <span
                class="border-default-200 bg-content1 flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-full border"
              >
                <img
                  v-if="app.logo_url && !failed[app.site_domain]"
                  :src="app.logo_url"
                  :alt="app.name"
                  class="size-full object-cover"
                  loading="lazy"
                  @error="onLogoError(app.site_domain)"
                />
                <span v-else class="text-default-500 text-[0.625rem] font-bold">
                  {{ initialOf(app.name) }}
                </span>
              </span>
              <span class="text-foreground min-w-0 flex-1 truncate text-sm">
                {{ app.name }}
              </span>
              <span
                v-if="app.auto_consent"
                class="bg-primary/10 text-primary shrink-0 rounded-full px-2 py-0.5 text-[0.625rem] font-medium"
              >
                官方
              </span>
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
