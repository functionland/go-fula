package blox

import (
	"context"
	"time"
)

// poolReconcileTimeout bounds one reconcile pass (two chain reads, each with retries).
const poolReconcileTimeout = 60 * time.Second

// reconcilePool clears the configured pool when the chain definitively says this Blox left it (see
// blockchain.ReconcilePoolConfig). Read errors keep the config; the next pass (startup or the 6 h loop) retries.
func (p *Blox) reconcilePool(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, poolReconcileTimeout)
	defer cancel()
	cleared, err := p.bl.ReconcilePoolConfig(ctx)
	if err != nil {
		log.Warnw("Pool reconcile skipped; keeping the configured pool", "err", err)
		return
	}
	if cleared {
		p.topicName = "0"
		p.chainName = ""
	}
}
