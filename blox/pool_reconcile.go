package blox

import (
	"context"
	"time"
)

const (
	// poolReconcileTimeout bounds one reconcile pass (two confirmed rounds of chain reads, each with retries).
	poolReconcileTimeout = 3 * time.Minute
	// poolReconcileStartupDelay keeps the first pass off the startup path and lets the network settle after a boot.
	poolReconcileStartupDelay = 2 * time.Minute
)

// reconcilePool clears the configured pool when the chain definitively says this Blox left it (see
// blockchain.ReconcilePoolConfig); the services restart that follows reloads the pool state. Read errors keep the
// config; the next pass (the 6 h loop) retries.
func (p *Blox) reconcilePool(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, poolReconcileTimeout)
	defer cancel()
	cleared, err := p.bl.ReconcilePoolConfig(ctx)
	if err != nil {
		log.Warnw("Pool reconcile skipped; keeping the configured pool", "err", err)
		return
	}
	if cleared {
		log.Info("Pool reconcile cleared the configured pool; fula services restart requested")
	}
}

// startPoolReconcile runs the first reconcile pass in the background, poolReconcileStartupDelay after start. Pool
// hosts are skipped: their own pool is not a membership the chain lists for them.
func (p *Blox) startPoolReconcile(ctx context.Context) {
	if p.poolHostMode {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		select {
		case <-ctx.Done():
			return
		case <-time.After(poolReconcileStartupDelay):
		}
		p.reconcilePool(ctx)
	}()
}
