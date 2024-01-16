package blox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/functionland/go-fula/announcements"
	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/common"
	"github.com/functionland/go-fula/exchange"
	"github.com/functionland/go-fula/ping"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("fula/blox")

type (
	Blox struct {
		ctx    context.Context
		cancel context.CancelFunc

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
		ping.WithWg(p.wg),
		ping.WithTimeout(3),
		ping.WithCount(p.pingCount))
	if err != nil {
		return nil, err
	}

	p.an, err = announcements.NewFxAnnouncements(p.h,
		announcements.WithAnnounceInterval(5),
		announcements.WithTimeout(3),
		announcements.WithTopicName(p.topicName),
		announcements.WithWg(p.wg),
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
		blockchain.WithTimeout(65),
		blockchain.WithWg(p.wg),
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

func (p *Blox) StoreCid(ctx context.Context, l ipld.Link, limit int) error {
	exists, err := p.Has(ctx, l)
	if err != nil {
		log.Errorw("And Error happened in checking Has", "err", err)
	}
	if exists {
		log.Warnw("link already exists in datastore", "l", l.String())
		return fmt.Errorf("link already exists in datastore %s", l.String())
	}
	providers, err := p.ex.FindProvidersIpni(l, p.relays)
	if err != nil {
		log.Errorw("And Error happened in StoreCid", "err", err)
		return err
	}
	// Iterate over the providers and ping
	for _, provider := range providers {
		if provider.ID != p.h.ID() {
			//Found a storer, now pull the cid
			//TODO: Ideally we should have a mechanism to reserve the pull requests and keep the pulled+requests to a max of replication factor
			log.Debugw("Found a storer", "id", provider.ID)

			replicas := len(providers)
			log.Debugw("Checking replicas vs limit", "replicas", replicas, "limit", limit)
			if replicas < limit {
				addrStr := "/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835/p2p-circuit/p2p/" + provider.ID.String()
				addr, err := multiaddr.NewMultiaddr(addrStr)
				if err != nil {
					log.Errorw("Failed to create multiaddress", "err", err)
					continue
				}
				p.h.Peerstore().AddAddrs(provider.ID, []multiaddr.Multiaddr{addr}, peerstore.ConnectedAddrTTL)
				log.Debugw("Started Pull in StoreCid", "from", provider.ID, "l", l)
				err = p.ex.PullBlock(ctx, provider.ID, l)
				if err != nil {
					log.Errorw("Error happened in pulling from provider", "err", err)
					continue
				}
				log.Debugw("link might be successfully stored", "l", l, "from", provider.ID)
				return nil
			} else {
				return fmt.Errorf("limit of %d is reached for %s", limit, l.String())
			}
		} else {
			log.Warnw("provider is the same as requestor", "l", l.String())
			return fmt.Errorf("provider is the same as requestor for link %s", l.String())
		}
	}
	return errors.New("no provider found")
}

func (p *Blox) StoreManifest(ctx context.Context, links []blockchain.LinkWithLimit, maxCids int) error {
	log.Debugw("StoreManifest", "links", links)
	var storedLinks []ipld.Link // Initialize an empty slice for successful storage
	for _, l := range links {
		if len(storedLinks) >= maxCids {
			break
		}
		exists, err := p.Has(ctx, l.Link)
		if err != nil {
			continue
		}
		if exists {
			continue
		}
		err = p.StoreCid(ctx, l.Link, l.Limit) // Assuming StoreCid is a function that stores the link and returns an error if it fails
		if err != nil {
			// If there's an error, log it and continue with the next link
			log.Errorw("Error storing CID", "link", l, "err", err)
			continue // Skip appending this link to the storedLinks slice
		}
		// Append to storedLinks only if StoreCid is successful but since Pull is async we need t0 check for existence of link before submitting to blockchain
		storedLinks = append(storedLinks, l.Link)
	}
	log.Debugw("StoreManifest", "storedLinks", storedLinks)
	// Handle the successfully stored links with the blockchain
	if len(storedLinks) > 0 {
		time.Sleep(2 * time.Minute)
		// Collect confirmed CIDs
		var hasLinks []ipld.Link
		for _, link := range storedLinks {
			h, err := p.Has(ctx, link)
			if err != nil || !h {
				continue
			}
			hasLinks = append(hasLinks, link)
		}
		log.Debugw("confirmed links done", "hasLinks", hasLinks)
		// Handle the successfully stored and confirmed links with the blockchain
		if len(hasLinks) > 0 {
			_, err := p.bl.HandleManifestBatchStore(ctx, p.topicName, hasLinks)
			if err != nil {
				log.Errorw("Error happened in storing manifest", "err", err)
				return err
			}
		}

		// If all links failed to store or confirm, return an error
		if len(hasLinks) == 0 {
			return errors.New("all links failed to confirm for storage")
		}
	}

	// If all links failed to store, return an error
	if len(storedLinks) == 0 {
		return errors.New("all links failed to store")
	}

	return nil
}

// FetchAvailableManifestsAndStore fetches available manifests and stores them.
func (p *Blox) FetchAvailableManifestsAndStore(ctx context.Context, maxCids int) error {
	childCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel() // It's good practice to call cancel to free resources if the childCtx finishes before the timeout
	// Fetch the available manifests for a specific pool_id
	availableLinks, err := p.bl.HandleManifestsAvailable(childCtx, p.topicName, maxCids)
	if err != nil {
		return fmt.Errorf("failed to fetch available manifests: %w", err)
	}
	log.Debugw("FetchAvailableManifestsAndStore", "availableLinks", availableLinks)
	// Check if there are enough manifests available
	if len(availableLinks) == 0 {
		return errors.New("no available manifests to store")
	}

	// Attempt to store the fetched manifests
	err = p.StoreManifest(childCtx, availableLinks, maxCids)
	if err != nil {
		return fmt.Errorf("failed to store manifests: %w", err)
	}
	return nil
}

func (p *Blox) Start(ctx context.Context) error {
	ctx = network.WithUseTransient(ctx, "fx.exchange")
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

	// Create an HTTP server instance
	p.IPFShttpServer = &http.Server{
		Addr:    "127.0.0.1:5001",
		Handler: p.ServeIpfsRpc(),
	}

	log.Debug("called wg.Add in blox start")
	p.wg.Add(1)
	go func() {
		log.Debug("called wg.Done in Start blox")
		defer p.wg.Done()
		defer log.Debug("Start blox go routine is ending")
		log.Infow("IPFS RPC server started on address http://127.0.0.1:5001")
		if err := p.IPFShttpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Errorw("IPFS RPC server stopped erroneously", "err", err)
		} else {
			log.Infow("IPFS RPC server stopped")
		}
	}()

	if anErr == nil {
		log.Debug("called wg.Add in AnnounceIExistPeriodically")
		p.wg.Add(1)
		go func() {
			log.Debug("called wg.Done in AnnounceIExistPeriodically")
			defer p.wg.Done() // Decrement the counter when the goroutine completes
			defer log.Debug("AnnounceIExistPeriodically go routine is ending")
			p.an.AnnounceIExistPeriodically(ctx)
		}()

		log.Debug("called wg.Add in HandleAnnouncements")
		p.wg.Add(1)
		go func() {
			log.Debug("called wg.Done in HandleAnnouncements")
			defer p.wg.Done() // Decrement the counter when the goroutine completes
			defer log.Debug("HandleAnnouncements go routine is ending")
			p.an.HandleAnnouncements(ctx)
		}()
	} else {
		log.Errorw("Announcement stopped erroneously", "err", anErr)
	}
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Starting a new goroutine for periodic task
	log.Debug("Called wg.Add in FetchAvailableManifestsAndStore")
	p.wg.Add(1)
	go func() {
		log.Debug("Called wg.Done in FetchAvailableManifestsAndStore")
		defer p.wg.Done()
		defer log.Debug("Start blox FetchAvailableManifestsAndStore go routine is ending")
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hasFreeSpace := true
				// This block will execute every 5 minutes
				freeSpace, err := wifi.GetBloxFreeSpace()
				if err != nil {
					if freeSpace.Size != 0 {
						if freeSpace.UsedPercentage > 90 {
							hasFreeSpace = false
						}
					}
				}
				if hasFreeSpace {
					if err := p.FetchAvailableManifestsAndStore(ctx, 50); err != nil {
						log.Errorw("Error in FetchAvailableManifestsAndStore", "err", err)
						// Handle the error or continue based on your requirement
					}
				}
			case <-ctx.Done():
				// This will handle the case where the parent context is canceled
				log.Info("Stopping periodic FetchAvailableManifestsAndStore due to context cancellation")
				return
			case <-p.ctx.Done():
				// This will handle the case where the parent context is canceled
				log.Info("Stopping periodic FetchAvailableManifestsAndStore due to context cancellation")
				return
			}
		}
	}()

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

func (p *Blox) PingDht(to peer.ID) error {
	//This is for unit testing and no need to call directly
	return p.ex.PingDht(to)
}

func (p *Blox) GetBlMembers() map[peer.ID]common.MemberStatus {
	//This is for unit testing and no need to call directly
	return p.bl.GetMembers()
}

func (p *Blox) BloxFreeSpace(ctx context.Context, to peer.ID) ([]byte, error) {
	//This is for unit testing and no need to call directly
	return p.bl.BloxFreeSpace(ctx, to)
}

func (p *Blox) EraseBlData(ctx context.Context, to peer.ID) ([]byte, error) {
	//This is for unit testing and no need to call directly
	return p.bl.EraseBlData(ctx, to)
}

func (p *Blox) StartAnnouncementServer(ctx context.Context) error {
	//This is for unit testing and no need to call directly
	err := p.an.Start(ctx, p.PubsubValidator)
	if err == nil {
		log.Debug("called wg.Add in StartAnnouncementServer1")
		p.wg.Add(1)
		go func() {
			log.Debug("called wg.Done in StartAnnouncementServer1")
			defer p.wg.Done() // Decrement the counter when the goroutine completes
			defer log.Debug("StartAnnouncementServer1 go routine is ending")
			p.an.AnnounceIExistPeriodically(ctx)
		}()
		log.Debug("called wg.Add in StartAnnouncementServer2")
		p.wg.Add(1)
		go func() {
			log.Debug("called wg.Done in StartAnnouncementServer2")
			defer p.wg.Done() // Decrement the counter when the goroutine completes
			defer log.Debug("StartAnnouncementServer2 go routine is ending")
			p.an.HandleAnnouncements(ctx)
		}()
	}
	return err
}

func (p *Blox) AnnounceJoinPoolRequestPeriodically(ctx context.Context) {
	log.Debug("Called wg.Add in AnnounceJoinPoolRequestPeriodically")
	p.wg.Add(1)
	//This is for unit testing and no need to call directly
	log.Debugf("AnnounceJoinPoolRequest ping count %d", p.pingCount)
	go func() {
		log.Debug("Called wg.Done in AnnounceJoinPoolRequestPeriodically")
		defer p.wg.Done() // Decrement the counter when the goroutine completes
		defer log.Debug("AnnounceJoinPoolRequestPeriodically go routine is ending")
		p.an.AnnounceJoinPoolRequestPeriodically(ctx)
	}()
}

func (p *Blox) ProvideLinkByDht(l ipld.Link) error {
	//This is for unit testing and no need to call directly
	log.Debug("ProvideLinkByDht test")
	return p.ex.ProvideDht(l)
}

func (p *Blox) FindLinkProvidersByDht(l ipld.Link) ([]peer.AddrInfo, error) {
	//This is for unit testing and no need to call directly
	log.Debug("FindLinkProvidersByDht test")
	return p.ex.FindProvidersDht(l)
}

func (p *Blox) UpdateDhtPeers(peers []peer.ID) error {
	//This is for unit testing and no need to call directly
	log.Debug("UpdateDhtPeers test")
	return p.ex.UpdateDhtPeers(peers)
}

func (p *Blox) Shutdown(ctx context.Context) error {
	log.Info("Shutdown in progress")

	// Cancel the context to signal all operations to stop
	p.cancel()

	// Shutdown the various components
	if bErr := p.bl.Shutdown(ctx); bErr != nil {
		log.Errorw("Error occurred in blockchain shutdown", "bErr", bErr)
		return bErr
	} else {
		log.Debug("Blockchain shutdown done")
	}
	if xErr := p.ex.Shutdown(ctx); xErr != nil {
		log.Errorw("Error occurred in exchange shutdown", "xErr", xErr)
		return xErr
	} else {
		log.Debug("exchange shutdown done")
	}
	if pErr := p.pn.Shutdown(ctx); pErr != nil {
		log.Errorw("Error occurred in ping shutdown", "pErr", pErr)
		return pErr
	} else {
		log.Debug("ping shutdown done")
	}
	if tErr := p.an.Shutdown(ctx); tErr != nil {
		log.Errorw("Error occurred in announcements shutdown", "tErr", tErr)
		return tErr
	} else {
		log.Debug("announcements shutdown done")
	}

	// Shutdown the HTTP server
	if IPFSErr := p.IPFShttpServer.Shutdown(ctx); IPFSErr != nil {
		log.Errorw("Error shutting down IPFS HTTP server", "IPFSErr", IPFSErr)
	}

	if dsErr := p.ds.Close(); dsErr != nil {
		log.Errorw("Error occurred in datastore shutdown", "dsErr", dsErr)
		return dsErr
	} else {
		log.Debug("datastore shutdown done")
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.wg.Wait()
	}()

	select {
	case <-done:
		// Handle remaining errors after all goroutines have finished
		log.Debug("Shutdown done")
		return nil
	case <-ctx.Done():
		err := ctx.Err()
		log.Infow("Shutdown completed with timeout", "err", err)
		return err
	}
}
