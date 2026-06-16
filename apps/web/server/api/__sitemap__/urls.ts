import { buildSitemapUrls } from '../../utils/kunSitemapSources'

// Dynamic content URLs for the sitemap. @nuxtjs/sitemap pulls this in via its
// `sources` config and fetches it server-side at request time (runtime, never
// build time — the Docker build has no Go-API access).
//
// Cached because enumerating every list endpoint is many Go-API calls and
// crawlers hit the sitemap repeatedly. swr serves the last-good list during an
// API outage instead of emitting an empty sitemap.
//
// SSR reaches the Go API by its compose service name (apiBaseSsr), falling back
// to public.apiBase for local air dev. Both already include /api/v1.
export default defineCachedEventHandler(
  async () => {
    const cfg = useRuntimeConfig()
    const apiBase = cfg.apiBaseSsr || cfg.public.apiBase
    return await buildSitemapUrls(apiBase)
  },
  {
    name: 'sitemap-urls',
    getKey: () => 'all',
    swr: true,
    maxAge: 60 * 60 * 6, // rebuild the URL list at most once per 6h
    staleMaxAge: 60 * 60 * 24 * 7 // keep serving it up to 7d if the API is down
  }
)
