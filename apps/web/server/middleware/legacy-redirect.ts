// 301s for paths this site has published and then moved.
//
// Two families so far:
//
//   /about, /blog        → /doc     (merged into a single "doc" feature, see
//                                    migration 016; the old blog was flat & empty)
//   /tags/:id            → /galgame/tag/:id
//   /labels/:id          → /galgame/official/:id
//
// The taxonomy pair moved in wave 146, one week after /tags|/labels themselves
// went live on 07-29 — which is exactly why it is worth a 301 now rather than
// later: the URLs are barely indexed, so the redirect costs almost nothing
// today and would cost more every week it waited. The new shape puts entry
// pages under /galgame/ in the SINGULAR on both this site and kungal, so one
// URL describes a tag wherever you meet it.
//
// /patch/:id does NOT come through here either, for the same reason and more
// sharply: the old page number only resolves through patch_redirect, so the
// redirect needs a lookup and lives in pages/patch/[...slug].vue. Guessing
// /galgame/:id from it would serve a different game -- 9,519 of the old page
// ids are also a live catalog work id.
//
// The wiki-keyed shells (/tag/:id, /official/:id) do NOT come through here:
// they have to resolve an id in a different id space first, so they stay real
// pages — and they point straight at the final address, never at a path that
// redirects again.
const MOVES: [RegExp, string][] = [
  [/^\/tags\/(\d+)$/, '/galgame/tag/$1'],
  [/^\/labels\/(\d+)$/, '/galgame/official/$1']
]

export default defineEventHandler((event) => {
  // event.path carries the query string; split it off so the patterns match a
  // bare pathname, then put it back on the target — /tags/42?page=3 must land
  // on page 3, not page 1.
  const [pathname = '', query] = (event.path || '').split('?')
  const suffix = query ? `?${query}` : ''

  if (pathname === '/about' || pathname.startsWith('/about/')) {
    return sendRedirect(
      event,
      '/doc' + pathname.slice('/about'.length) + suffix,
      301
    )
  }
  if (pathname === '/blog' || pathname.startsWith('/blog/')) {
    return sendRedirect(event, '/doc', 301)
  }

  for (const [pattern, target] of MOVES) {
    if (pattern.test(pathname)) {
      return sendRedirect(event, pathname.replace(pattern, target) + suffix, 301)
    }
  }
})
