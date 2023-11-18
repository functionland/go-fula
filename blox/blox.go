package blox

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/functionland/go-fula/announcements"
	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/common"
	"github.com/functionland/go-fula/exchange"
	"github.com/functionland/go-fula/ping"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("fula/blox")

type (
	Blox struct {
		ctx    context.Context
		cancel context.CancelFunc
		wg     sync.WaitGroup

		*options
		ls ipld.LinkSystem
		ex *exchange.FxExchange
		bl *blockchain.FxBlockchain
		pn *ping.FxPing
		an *announcements.FxAnnouncements
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
		ping.WithWg(&p.wg),
		ping.WithTimeout(3),
		ping.WithCount(p.pingCount))
	if err != nil {
		return nil, err
	}

	p.an, err = announcements.NewFxAnnouncements(p.h,
		announcements.WithAnnounceInterval(5),
		announcements.WithTimeout(3),
		announcements.WithTopicName(p.topicName),
		announcements.WithWg(&p.wg),
		announcements.WithRelays(p.relays),
	)
	if err != nil {
		return nil, err
	}

	p.bl, err = blockchain.NewFxBlockchain(p.h, p.pn, p.an,
		blockchain.NewSimpleKeyStorer(""),
		blockchain.WithAuthorizer(authorizer),
		blockchain.WithAuthorizedPeers(authorizedPeers),
		blockchain.WithBlockchainEndPoint(p.blockchainEndpoint),
		blockchain.WithTimeout(30),
		blockchain.WithWg(&p.wg),
		blockchain.WithFetchFrequency(3),
		blockchain.WithTopicName(p.topicName),
		blockchain.WithUpdatePoolName(p.updatePoolName),
		blockchain.WithRelays(p.relays),
		blockchain.WithMaxPingTime(p.maxPingTime),
		blockchain.WithMinSuccessPingCount(p.minSuccessRate*p.pingCount/100),
	)
	if err != nil {
		return nil, err
	}

	p.an.SetPoolJoinRequestHandler(p.bl)

	return &p, nil
}

func (p *Blox) PubsubValidator(ctx context.Context, id peer.ID, msg *pubsub.Message) bool {
	status, exists := p.bl.GetMemberStatus(id)
	return p.an.ValidateAnnouncement(ctx, id, msg, status, exists)
}

func (p *Blox) Start(ctx context.Context) error {
	// implemented topic validators with chain integration.
	validator := p.PubsubValidator

	anErr := p.an.Start(ctx, validator)

	if err := p.ex.Start(ctx); err != nil {
		return err
	}
	if err := p.bl.Start(ctx); err != nil {
		return err
	}
	if err := p.bl.FetchUsersAndPopulateSets(ctx, p.topicName, true); err != nil {
		log.Errorw("FetchUsersAndPopulateSets failed", "err", err)
	}
	log.Debug("called wg.Add in blox start")
	p.wg.Add(1)
	go func() {
		log.Debug("called wg.Done in Start blox")
		defer p.wg.Done()
		log.Infow("IPFS RPC server started on address http://localhost:5001")
		switch err := http.ListenAndServe("localhost:5001", p.ServeIpfsRpc()); {
		case errors.Is(err, http.ErrServerClosed):
			log.Infow("IPFS RPC server stopped")
		default:
			log.Errorw("IPFS RPC server stopped erroneously", "err", err)
		}
	}()

	if anErr == nil {
		log.Debug("called wg.Add in blox start anErr1")
		p.wg.Add(1)
		go p.an.AnnounceIExistPeriodically(ctx)
		log.Debug("called wg.Add in blox start anErr2")

		p.wg.Add(1)
		go p.an.HandleAnnouncements(ctx)
	} else {
		log.Errorw("Announcement stopped erroneously", "err", anErr)
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	return nil
}

func (p *Blox) SetAuth(ctx context.Context, on peer.ID, subject peer.ID, allow bool) error {
	err := p.bl.SetAuth(ctx, on, subject, allow)
	if err != nil {
		return err
	}
	return p.ex.SetAuth(ctx, on, subject, allow)
}

func (p *Blox) StartPingServer(ctx context.Context) error {
	//This is for unit testing and no need to call directly
	return p.pn.Start(ctx)
}

func (p *Blox) Ping(ctx context.Context, to peer.ID) (int, int, error) {
	//This is for unit testing and no need to call directly
	return p.pn.Ping(ctx, to)
}

func (p *Blox) GetBlMembers() map[peer.ID]common.MemberStatus {
	//This is for unit testing and no need to call directly
	return p.bl.GetMembers()
}

func (p *Blox) StartAnnouncementServer(ctx context.Context) error {
	//This is for unit testing and no need to call directly
	p.wg.Add(1)
	err := p.an.Start(ctx, p.PubsubValidator)
	if err == nil {
		log.Debug("called wg.Add in StartAnnouncementServer1")
		p.wg.Add(1)
		go p.an.AnnounceIExistPeriodically(ctx)
		log.Debug("called wg.Add in StartAnnouncementServer2")
		p.wg.Add(1)
		go p.an.HandleAnnouncements(ctx)
	}
	return err
}

func (p *Blox) AnnounceJoinPoolRequestPeriodically(ctx context.Context) {
	p.wg.Add(1)
	//This is for unit testing and no need to call directly
	log.Debugf("AnnounceJoinPoolRequest ping count %d", p.pingCount)
	go p.an.AnnounceJoinPoolRequestPeriodically(ctx)
}

func (p *Blox) Shutdown(ctx context.Context) error {
	log.Info("Shutdown in progress")
	bErr := p.bl.Shutdown(ctx)
	log.Info("blockchain Shutdown in progress")
	xErr := p.ex.Shutdown(ctx)
	log.Info("exchange Shutdown in progress")
	pErr := p.pn.Shutdown(ctx)
	log.Info("ping Shutdown in progress")
	tErr := p.an.Shutdown(ctx)
	log.Info("announcement Shutdown in progress")
	p.cancel()
	dsErr := p.ds.Close()
	log.Info("datastore Shutdown in progress")
	done := make(chan struct{}, 1)
	go func() {
		defer close(done)
		p.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		if tErr != nil {
			log.Errorw("Error occurred in announcement shutdown", "tErr", tErr)
			return tErr
		}
		if dsErr != nil {
			log.Errorw("Error occurred in ds shutdown", "dsErr", dsErr)
			return dsErr
		}
		if bErr != nil {
			log.Errorw("Error occurred in blockchain shutdown", "bErr", bErr)
			return bErr
		}
		if pErr != nil {
			log.Errorw("Error occurred in Ping shutdown", "pErr", pErr)
			return pErr
		}
		return xErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
