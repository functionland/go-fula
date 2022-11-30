package pool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/functionland/go-fula/exchange"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

var log = logging.Logger("fula/pool")

type (
	Pool struct {
		ctx    context.Context
		cancel context.CancelFunc
		wg     sync.WaitGroup

		*options
		sub   *pubsub.Subscription
		topic *pubsub.Topic
		ls    ipld.LinkSystem
		ex    *exchange.Exchange
	}
)

func New(o ...Option) (*Pool, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	p := Pool{
		options: opts,
		ls:      cidlink.DefaultLinkSystem(),
	}
	p.ls.StorageReadOpener = p.blockReadOpener
	p.ls.StorageWriteOpener = p.blockWriteOpener
	p.ex = exchange.New(p.h, p.ls)
	return &p, nil
}

func (p *Pool) Start(ctx context.Context) error {
	if err := p.ex.Start(ctx); err != nil {
		return err
	}
	gsub, err := pubsub.NewGossipSub(ctx, p.h,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
	)
	if err != nil {
		return err
	}
	// TODO: implement topic validators once there is some chain integration.
	p.topic, err = gsub.Join(p.topicName)
	if err != nil {
		return err
	}
	p.sub, err = p.topic.Subscribe()
	if err != nil {
		return err
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.announceIExistPeriodically()
	go p.handleAnnouncements()
	return nil
}

func (p *Pool) handleAnnouncements() {
	p.wg.Add(1)
	defer p.wg.Done()
	for {
		msg, err := p.sub.Next(p.ctx)
		switch {
		case p.ctx.Err() != nil || err == pubsub.ErrSubscriptionCancelled:
			log.Info("stopped handling announcements")
			return
		case err != nil:
			log.Errorw("failed to get the next announcement", "err", err)
			continue
		}
		from, err := peer.IDFromBytes(msg.From)
		if err != nil {
			log.Errorw("failed to decode announcement sender", "err", err)
			continue
		}
		if from == p.h.ID() {
			continue
		}
		a := &Announcement{}
		if err = a.UnmarshalBinary(msg.Data); err != nil {
			log.Errorw("failed to decode announcement data", "err", err)
			continue
		}
		addrs, err := a.GetAddrs()
		if err != nil {
			log.Errorw("failed to decode announcement addrs", "err", err)
			continue
		}
		p.h.Peerstore().AddAddrs(from, addrs, peerstore.PermanentAddrTTL)
		log.Infow("received announcement", "from", from, "self", p.h.ID(), "announcement", a)
	}
}

func (p *Pool) announceIExistPeriodically() {
	p.wg.Add(1)
	defer p.wg.Done()
	ticker := time.NewTicker(p.announceInterval)
	for {
		select {
		case <-p.ctx.Done():
			log.Info("stopped making periodic announcements")
			return
		case t := <-ticker.C:
			a := &Announcement{
				Version: Version0,
				Type:    IExistAnnouncementType,
			}
			a.SetAddrs(p.h.Addrs()...)
			b, err := a.MarshalBinary()
			if err != nil {
				log.Errorw("failed to encode iexist announcement", "err", err)
				continue
			}
			if err := p.topic.Publish(p.ctx, b); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Info("stopped making periodic announcements")
					return
				}
				log.Errorw("failed to publish iexist announcement", "err", err)
				continue
			}
			log.Infow("announced iexist message", "from", p.h.ID(), "announcement", a, "time", t)
		}
	}
}

func (p *Pool) Shutdown(ctx context.Context) error {
	p.ex.Shutdown()
	p.sub.Cancel()
	err := p.topic.Close()
	p.cancel()
	done := make(chan struct{}, 1)
	go func() {
		defer close(done)
		p.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
