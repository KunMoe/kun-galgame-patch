package storelink

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"kun-galgame-patch-api/pkg/storeclient"
	"kun-galgame-patch-api/pkg/upstream"

	"gorm.io/gorm/clause"
)

const (
	queueSize = 1024
	// The default developer-key tier is 60 requests a minute. Pacing at 40 keeps
	// a whole minute of headroom for the campaign probe and for a burst the
	// limiter counted in the previous window.
	mintInterval = 1500 * time.Millisecond
	mintTimeout  = 10 * time.Second
	// A site whose products are all minted mints nothing for days, so the
	// campaign has to be re-read on its own clock: it is only ever learned from
	// a purchase-links answer.
	campaignInterval = 6 * time.Hour
)

// Start loads the cache and runs the minter until the returned func is called.
// It is nil-safe and a no-op when the store face is unconfigured — the affiliate
// template alone is a complete, working feature.
func (r *Resolver) Start() func() {
	if !r.Configured() {
		return func() {}
	}
	r.load()
	slog.Info("dlsite 短链已启用", "cached_links", r.Count())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.refreshCampaign(ctx)
		r.run(ctx)
	}()
	return func() {
		cancel()
		<-done
	}
}

func (r *Resolver) refreshCampaign(ctx context.Context) {
	if r.stopped() {
		return
	}
	probe := r.probeProduct()
	if probe == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, mintTimeout)
	defer cancel()
	links, err := r.client.PurchaseLinks(ctx, probe)
	if err != nil {
		slog.Warn("dlsite 优惠券活动刷新失败, 沿用上一次的结果", "error", err)
		return
	}
	r.setCampaign(links.Campaign, links.CouponURL)
}

func (r *Resolver) run(ctx context.Context) {
	pace := time.NewTimer(mintInterval)
	defer pace.Stop()
	if !pace.Stop() {
		<-pace.C
	}
	campaign := time.NewTicker(campaignInterval)
	defer campaign.Stop()

	for {
		var workno string
		select {
		case <-ctx.Done():
			return
		case <-campaign.C:
			r.refreshCampaign(ctx)
			continue
		case workno = <-r.queue:
		}

		pace.Reset(r.mint(ctx, workno))
		select {
		case <-ctx.Done():
			return
		case <-pace.C:
		}
	}
}

func (r *Resolver) enqueue(workno string) {
	if !r.Configured() {
		return
	}
	r.pendMu.Lock()
	_, isBad := r.bad[workno]
	_, isQueued := r.queued[workno]
	if r.halted || isBad || isQueued {
		r.pendMu.Unlock()
		return
	}
	r.queued[workno] = struct{}{}
	r.pendMu.Unlock()

	select {
	case r.queue <- workno:
	default:
		// Queue full. Dropping is safe: the next reader of this game enqueues it
		// again, and a full queue means the minter is already saturated.
		r.release(workno)
	}
}

func (r *Resolver) release(workno string) {
	r.pendMu.Lock()
	delete(r.queued, workno)
	r.pendMu.Unlock()
}

func (r *Resolver) stopped() bool {
	r.pendMu.Lock()
	defer r.pendMu.Unlock()
	return r.halted
}

func (r *Resolver) mint(ctx context.Context, workno string) time.Duration {
	defer r.release(workno)
	if r.stopped() {
		return mintInterval
	}

	ctx, cancel := context.WithTimeout(ctx, mintTimeout)
	defer cancel()
	links, err := r.client.PurchaseLinks(ctx, workno)
	if err != nil {
		return r.mintFailed(workno, err, time.Now())
	}

	row := Link{ProductID: links.ProductID, ShortURL: links.PurchaseURL}
	// DoNothing, not DoUpdates: infra pins one alias per (client, product)
	// forever and refuses to re-mint, so a row that already exists is the alias
	// the shortener is counting. Overwriting it would move attribution to a link
	// nobody clicked.
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "product_id"}}, DoNothing: true,
	}).Create(&row).Error; err != nil {
		slog.Error("写入 dlsite 短链失败", "workno", workno, "error", err)
		return mintInterval
	}
	r.remember(row.ProductID, row.ShortURL)
	r.setCampaign(links.Campaign, links.CouponURL)
	return mintInterval
}

func (r *Resolver) mintFailed(workno string, err error, now time.Time) time.Duration {
	switch {
	case errors.Is(err, storeclient.ErrQuotaExceeded), errors.Is(err, storeclient.ErrUnauthorized):
		r.halt()
		slog.Error("dlsite 短链铸造已停止, 新作品回落联盟直链(丢失归因); 提高 infra 的 per-client 配额或修 store:read scope 后重启生效",
			"workno", workno, "minted", r.Count(), "error", err)
	case errors.Is(err, storeclient.ErrInvalidProduct):
		r.markBad(workno)
		slog.Warn("store 面拒绝该 workno, 不再重试", "workno", workno, "error", err)
	case errors.Is(err, storeclient.ErrRateLimited):
		wait := rateLimitWait(err, now)
		slog.Warn("dlsite 短链铸造被限流, 暂停到 Retry-After", "workno", workno, "resume_in", wait, "error", err)
		return wait
	default:
		slog.Warn("dlsite 短链铸造失败, 下一次浏览会重试", "workno", workno, "error", err)
	}
	return mintInterval
}

// A spent daily quota used to count as a transient failure, so the minter
// retried it every 1.5s until the UTC day rolled over. Infra sends Retry-After
// on both limits; without it, a quota waits for the next UTC day and a rate
// limit for the next minute.
func rateLimitWait(err error, now time.Time) time.Duration {
	e, _ := upstream.As(err)
	if e != nil && e.RetryAfter > 0 {
		return e.RetryAfter
	}
	if e != nil && e.Code == "QUOTA_EXCEEDED" {
		u := now.UTC()
		return time.Date(u.Year(), u.Month(), u.Day()+1, 0, 0, 0, 0, time.UTC).Sub(now)
	}
	return time.Minute
}

func (r *Resolver) halt() {
	r.pendMu.Lock()
	r.halted = true
	r.pendMu.Unlock()
}

func (r *Resolver) markBad(workno string) {
	r.pendMu.Lock()
	r.bad[workno] = struct{}{}
	r.pendMu.Unlock()
}
