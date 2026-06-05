// 301 the legacy /about and /blog URLs to the unified /doc.
//
// /about and /blog were merged into a single "doc" feature (see migration 016).
// Preserve inbound links / SEO: /about/<category>/<slug> → /doc/<category>/<slug>,
// /about → /doc, and any /blog* → /doc (the old blog was flat & empty).
export default defineEventHandler((event) => {
  const path = event.path || ''
  if (path === '/about' || path.startsWith('/about/')) {
    return sendRedirect(event, '/doc' + path.slice('/about'.length), 301)
  }
  if (path === '/blog' || path.startsWith('/blog/')) {
    return sendRedirect(event, '/doc', 301)
  }
})
