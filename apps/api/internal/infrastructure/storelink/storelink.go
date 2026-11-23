// Package storelink resolves a DLsite workno to the purchase link the reader
// should follow, preferring the short link infra minted for this site over the
// bare affiliate template.
//
// The short link is what carries attribution: the click counter behind it is
// what settlement reads, so a direct affiliate URL still sells the game but the
// sale is invisible. Minting is therefore worth doing — but never on the render
// path. A (client, product) pair maps to one alias forever, so the whole table
// is held in memory and a miss returns the template link while the background
// minter fills the gap for the next reader.
package storelink

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"kun-galgame-patch-api/pkg/storeclient"

	"gorm.io/gorm"
)

type Link struct {
	ProductID string `gorm:"primaryKey;size:10"`
	ShortURL  string `gorm:"type:text;not null"`
	CreatedAt time.Time
}

func (Link) TableName() string { return "dlsite_store_link" }

const worknoPlaceholder = "{workno}"

type Options struct {
	DB     *gorm.DB
	Client *storeclient.Client
	// LinkTemplate is the bare affiliate template (site setting
	// dlsite_link_template), read per call. It stays the fallback for every
	// product with no short link yet, and the whole feature when the store face
	// is unconfigured or refuses.
	LinkTemplate func() string
	// StaticCoupon (site setting dlsite_coupon_url) is shown when the store
	// face reports no running campaign. It is an independent landing page, not a
	// campaign, so "no campaign" must not blank out the coupon the site already
	// offers.
	StaticCoupon func() string
}

func Fixed(v string) func() string { return func() string { return v } }

func read(f func() string) string {
	if f == nil {
		return ""
	}
	return f()
}

type Resolver struct {
	db           *gorm.DB
	client       *storeclient.Client
	tmpl         func() string
	staticCoupon func() string

	mu       sync.RWMutex
	links    map[string]string
	coupon   string
	campaign string

	queue  chan string
	pendMu sync.Mutex
	queued map[string]struct{}
	bad    map[string]struct{}
	halted bool
}

func New(opts Options) *Resolver {
	return &Resolver{
		db:           opts.DB,
		client:       opts.Client,
		tmpl:         opts.LinkTemplate,
		staticCoupon: opts.StaticCoupon,
		links:        map[string]string{},
		queue:        make(chan string, queueSize),
		queued:       map[string]struct{}{},
		bad:          map[string]struct{}{},
	}
}

// Links is what one game's purchase entry renders. CampaignName is empty
// whenever CouponURL is the static fallback rather than a running campaign,
// which is what tells the frontend to keep its own description of that page.
type Links struct {
	PurchaseURL  string
	CouponURL    string
	CampaignName string
}

// Resolve is nil-safe and never blocks: a handler wired without a resolver, or
// a game with no DLsite workno, renders no purchase entry at all.
func (r *Resolver) Resolve(workno string) Links {
	if r == nil || !storeclient.ValidProductID(workno) {
		return Links{}
	}

	r.mu.RLock()
	short, minted := r.links[workno]
	out := Links{CouponURL: r.coupon, CampaignName: r.campaign}
	r.mu.RUnlock()
	if out.CouponURL == "" {
		out.CouponURL, out.CampaignName = read(r.staticCoupon), ""
	}

	if minted {
		out.PurchaseURL = short
	} else {
		r.enqueue(workno)
		out.PurchaseURL = templateLink(read(r.tmpl), workno)
	}
	if out.PurchaseURL == "" {
		return Links{}
	}
	return out
}

func templateLink(template, workno string) string {
	if template == "" {
		return ""
	}
	return strings.ReplaceAll(template, worknoPlaceholder, workno)
}

// Configured reports whether short links are in play at all; the affiliate
// template can still be on without it.
func (r *Resolver) Configured() bool {
	return r != nil && r.client.Configured() && r.db != nil
}

func (r *Resolver) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.links)
}

func (r *Resolver) load() {
	var rows []Link
	if err := r.db.Find(&rows).Error; err != nil {
		slog.Error("载入 dlsite 短链缓存失败, 本次启动全部回落联盟直链", "error", err)
		return
	}
	links := make(map[string]string, len(rows))
	for _, row := range rows {
		links[row.ProductID] = row.ShortURL
	}
	r.mu.Lock()
	r.links = links
	r.mu.Unlock()
}

func (r *Resolver) remember(productID, shortURL string) {
	r.mu.Lock()
	r.links[productID] = shortURL
	r.mu.Unlock()
}

// setCampaign assigns unconditionally: every purchase-links answer restates the
// campaign, so an ended campaign has to clear the link it opened.
func (r *Resolver) setCampaign(c *storeclient.Campaign, couponURL string) {
	name := ""
	if c != nil {
		name = c.Name
	}
	r.mu.Lock()
	r.coupon, r.campaign = couponURL, name
	r.mu.Unlock()
}

// probeProduct is a product already minted, used to re-read the campaign
// without spending quota on a new alias. Lowest id so the probe is stable.
func (r *Resolver) probeProduct() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lowest := ""
	for id := range r.links {
		if lowest == "" || id < lowest {
			lowest = id
		}
	}
	return lowest
}
