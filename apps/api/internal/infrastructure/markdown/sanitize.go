package markdown

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// sanitizer is the single server-side XSS boundary applied to goldmark's HTML
// output. goldmark now runs with html.WithUnsafe() (raw user HTML passes
// through), so the rendered result is UNTRUSTED and this allow-list is what
// makes it safe to bind via v-html on the frontend (which has no client-side
// sanitizer). It strips <script>/<style>, event handlers (onerror, onload, …),
// and unsafe URL schemes (javascript:/vbscript:/data:) while keeping everything
// this package's renderers legitimately emit. See newSanitizePolicy.
var sanitizer = newSanitizePolicy()

// Sanitize runs the shared allow-list over already-rendered HTML. Exposed so the
// render entry points (Render / RenderWithTOC) share one boundary.
func Sanitize(html string) string { return sanitizer.Sanitize(html) }

// newSanitizePolicy builds moyu's allow-list on top of bluemonday's UGCPolicy,
// extended with exactly what this package's renderers emit. Mirrors forum's
// shared whitelist strategy (docs/todos §3) so the two sites sanitize alike.
func newSanitizePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// Internal SPA links: moyu user content routinely uses relative hrefs
	// (/patch/:id, /user/:id) and the /image/<hash> content-image token before
	// resolution. UGCPolicy drops relative URLs by default — allow them so
	// internal links and unresolved image tokens survive. javascript:/data:
	// carry a scheme, so they are still rejected by the scheme allow-list.
	p.AllowRelativeURLs(true)

	// class carries no script; required by the @mention chip (kun-mention) and
	// goldmark's fenced-code `language-*` classes (frontend prose theming).
	p.AllowAttrs("class").Globally()

	// Auto heading-id anchors (parser.WithAutoHeadingID) → TOC fragment links.
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")

	// @mention link: <a class="kun-mention" data-id="123" href="/user/123/...">.
	p.AllowAttrs("data-id").OnElements("a")

	// Content-image metadata (contentImageTransformer): width/height reserve the
	// aspect ratio (no layout shift), data-thumbhash drives the blur-up. Value-
	// constrained (digits / base64) so nothing else can ride these attributes.
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]+$`)).OnElements("img")
	p.AllowAttrs("data-thumbhash").Matching(regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)).OnElements("img")

	// GFM task-list checkboxes: <input type="checkbox" checked disabled />.
	p.AllowElements("input")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").OnElements("input")

	// GFM table cell alignment.
	p.AllowAttrs("align").OnElements("td", "th")

	return p
}
