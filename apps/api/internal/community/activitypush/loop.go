package activitypush

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	busyPause     = 250 * time.Millisecond
	idlePoll      = 5 * time.Second
	backoffStart  = 5 * time.Second
	backoffCap    = 5 * time.Minute
	tickTimeout   = 2 * time.Minute
	reconcileSpec = "45 4 * * *"
)

// Origin is the https scheme://host of the OAuth redirect URI, or "" for any
// other. Community accepts an item URL only on a host registered for this
// client, and a development redirect (http://127.0.0.1:…) must never reach
// the production feed.
func Origin(redirectURI string) string {
	u, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// Start runs the drainer and the daily reconcile until the returned stop is
// called.
func (p *Pusher) Start() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() { p.drainLoop(ctx) })

	c := cron.New(cron.WithLocation(beijing))
	job := cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
		report, err := p.Reconcile(ctx)
		if err != nil {
			slog.Error("activity reconcile failed", "error", err, "enqueued", report.Enqueued)
			return
		}
		slog.Info("activity reconcile done", "stored", report.Stored, "local", report.Local, "enqueued", report.Enqueued)
	}))
	if _, err := c.AddJob(reconcileSpec, job); err != nil {
		slog.Error("activity reconcile not scheduled", "error", err)
	}
	c.Start()
	slog.Info("activity push started", "origin", p.resolver.origin)

	return func() {
		cancel()
		<-c.Stop().Done()
		wg.Wait()
	}
}

func (p *Pusher) drainLoop(ctx context.Context) {
	backoff := time.Duration(0)
	for {
		wait := idlePoll
		tctx, cancel := context.WithTimeout(ctx, tickTimeout)
		n, err := p.RunOnce(tctx)
		cancel()
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			backoff = min(max(backoff*2, backoffStart), backoffCap)
			wait = backoff
			slog.Warn("activity push failed; backing off", "error", err, "retry_in", wait)
		default:
			backoff = 0
			if n > 0 {
				wait = busyPause
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}
