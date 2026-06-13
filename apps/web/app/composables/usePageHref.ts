// Builds a crawlable href for a pagination page on a URL-page-driven list.
//
// KunPagination renders real `<a href>` (→ config.linkComponent / NuxtLink) when
// given `page-href`, instead of buttons — so paginated lists become crawlable by
// search engines and progressively enhanced (work without JS). KunUI 0.12.0.
//
// The returned href preserves the current route's path AND every other query
// param (filters / sort / tag-or-official id), overriding only `page`. page 1
// drops the `page` param so the first page stays the canonical, clean URL.
//
// Only use on pages whose `page` is already read from `route.query.page`
// (galgame / resource / comment / tag / official). On local-`ref` pagination
// (admin tables, user tabs) an href wouldn't actually drive the page, so those
// keep the button pagination.
export const usePageHref = () => {
  const route = useRoute()
  return (p: number): string => {
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(route.query)) {
      if (key === 'page') continue
      if (Array.isArray(value)) {
        for (const v of value) if (v != null) params.append(key, String(v))
      } else if (value != null) {
        params.set(key, String(value))
      }
    }
    if (p > 1) params.set('page', String(p))
    const qs = params.toString()
    return qs ? `${route.path}?${qs}` : route.path
  }
}
