package blox

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

var log = logging.Logger("fula/blox")

type (
	Blox struct {
		ctx    context.Context
		cancel context.CancelFunc
		wg     sync.WaitGroup

		*options
		sub   *pubsub.Subscription
		topic *pubsub.Topic
		ls    ipld.LinkSystem
		ex    *exchange.FxExchange
		bl    *blockchain.FxBlockchain
	}
)

func New(o ...Option) (*Blox, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	p := Blox{
		options: opts,
		ls:      cidlink.DefaultLinkSystem(),
	}
	p.ls.StorageReadOpener = p.blockReadOpener
	p.ls.StorageWriteOpener = p.blockWriteOpener
	p.ex, err = exchange.NewFxExchange(p.h, p.ls,
		exchange.WithAuthorizer(p.authorizer),
		exchange.WithAllowTransientConnection(p.allowTransientConnection))
	if err != nil {
		return nil, err
	}
	p.bl, err = blockchain.NewFxBlockchain(p.h,
		blockchain.NewSimpleKeyStorer(),
		blockchain.WithAuthorizer(p.authorizer),
		blockchain.WithAllowTransientConnection(p.allowTransientConnection),
		blockchain.WithBlockchainEndPoint("127.0.0.1:4000"),
		blockchain.WithTimeout(30))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Blox) Start(ctx context.Context) error {
	if err := p.ex.Start(ctx); err != nil {
		return err
	}
	if err := p.bl.Start(ctx); err != nil {
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

func (p *Blox) SetAuth(ctx context.Context, on peer.ID, subject peer.ID, allow bool) error {
	err := p.bl.SetAuth(ctx, on, subject, allow)
	if err != nil {
		return err
	}
	return p.ex.SetAuth(ctx, on, subject, allow)
}

func (p *Blox) handleAnnouncements() {
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

func (p *Blox) announceIExistPeriodically() {
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

func (p *Blox) Shutdown(ctx context.Context) error {
	bErr := p.bl.Shutdown(ctx)
	xErr := p.ex.Shutdown(ctx)
	p.sub.Cancel()
	tErr := p.topic.Close()
	p.cancel()
	dsErr := p.ds.Close()
	done := make(chan struct{}, 1)
	go func() {
		defer close(done)
		p.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		if tErr != nil {
			return tErr
		}
		if dsErr != nil {
			return dsErr
		}
		if bErr != nil {
			return bErr
		}
		return xErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
