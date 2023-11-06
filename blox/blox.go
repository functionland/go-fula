package blox

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	"github.com/functionland/go-fula/ping"
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
		pn    *ping.FxPing
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
	p.ex, err = exchange.NewFxExchange(p.h, p.ls, p.exchangeOpts...)
	if err != nil {
		return nil, err
	}
	authorizer, err := p.ex.GetAuth(p.ctx)
	if err != nil {
		authorizer = opts.authorizer
	}
	authorizedPeers, err := p.ex.GetAuthorizedPeers(p.ctx)
	if err != nil {
		authorizedPeers = opts.authorizedPeers
	}
	p.pn, err = ping.NewFxPing(p.h,
		ping.WithAllowTransientConnection(true),
		ping.WithTimeout(3))
	if err != nil {
		return nil, err
	}

	p.bl, err = blockchain.NewFxBlockchain(p.h, p.pn,
		blockchain.NewSimpleKeyStorer(""),
		blockchain.WithAuthorizer(authorizer),
		blockchain.WithAuthorizedPeers(authorizedPeers),
		blockchain.WithBlockchainEndPoint("127.0.0.1:4000"),
		blockchain.WithTimeout(30))
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (p *Blox) validateAnnouncement(ctx context.Context, id peer.ID, msg *pubsub.Message) bool {
	a := &Announcement{}
	if err := a.UnmarshalBinary(msg.Data); err != nil {
		log.Errorw("failed to unmarshal announcement data", "err", err)
		return false
	}
	status, exists := p.bl.GetMemberStatus(id)

	switch a.Type {
	case NewManifestAnnouncementType:
		// Check if sender is approved
		if !exists {
			log.Errorw("peer is not recognized", "peer", id)
			return false
		}
		if status != blockchain.Approved {
			log.Errorw("peer is not an approved member", "peer", id)
			return false
		}
	case PoolJoinRequestAnnouncementType, PoolJoinApproveAnnouncementType, IExistAnnouncementType:
		// Any member status is valid for a pool join announcement
	default:
		return false
	}

	// If all checks pass, the message is valid.
	return true
}

func (p *Blox) Start(ctx context.Context) error {
	if err := p.ex.Start(ctx); err != nil {
		return err
	}
	if err := p.bl.Start(ctx); err != nil {
		return err
	}
	p.bl.FetchUsersAndPopulateSets(ctx, p.topicName)
	go func() {
		log.Infow("IPFS RPC server started on address http://localhost:5001")
		switch err := http.ListenAndServe("localhost:5001", p.ServeIpfsRpc()); {
		case errors.Is(err, http.ErrServerClosed):
			log.Infow("IPFS RPC server stopped")
		default:
			log.Errorw("IPFS RPC server stopped erroneously", "err", err)
		}
	}()

	// TODO: implement topic validators once there is some chain integration.
	validator := func(ctx context.Context, id peer.ID, msg *pubsub.Message) bool {
		return p.validateAnnouncement(ctx, id, msg)
	}

	gsub, err := pubsub.NewGossipSub(ctx, p.h,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageSigning(true),
		pubsub.WithDefaultValidator(validator),
	)
	if err != nil {
		return err
	}

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
	pErr := p.pn.Shutdown(ctx)
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
		if pErr != nil {
			return pErr
		}
		return xErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
