package blox

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/functionland/go-fula/announcements"
	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/common"
	"github.com/functionland/go-fula/exchange"
	"github.com/functionland/go-fula/ping"
	"github.com/ipfs-cluster/ipfs-cluster/api"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/client/rpc"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
)

var log = logging.Logger("fula/blox")

type (
	Blox struct {
		ctx    context.Context
		cancel context.CancelFunc

		*options
		ls   ipld.LinkSystem
		ex   *exchange.FxExchange
		bl   *blockchain.FxBlockchain
		pn   *ping.FxPing
		an   *announcements.FxAnnouncements
		once sync.Once
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
		once:    sync.Once{},
	}
	p.ls.StorageReadOpener = p.blockReadOpener
	p.ls.StorageWriteOpener = p.blockWriteOpener
	if opts.ls != nil {
		p.ls = *opts.ls
	}
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
	log.Debug("Ping server started successfully")
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
		blockchain.NewSimpleKeyStorer(p.secretsPath),
		blockchain.WithAuthorizer(authorizer),
		blockchain.WithAuthorizedPeers(authorizedPeers),
		blockchain.WithBlockchainEndPoint(p.blockchainEndpoint),
		blockchain.WithSecretsPath(p.secretsPath),
		blockchain.WithTimeout(65),
		blockchain.WithWg(p.wg),
		blockchain.WithFetchFrequency(1*time.Minute),
		blockchain.WithTopicName(p.topicName),
		blockchain.WithUpdatePoolName(p.updatePoolName),
		blockchain.WithGetPoolName(p.getPoolName),
		blockchain.WithRelays(p.relays),
		blockchain.WithMaxPingTime(p.maxPingTime),
		blockchain.WithIpfsClient(p.rpc),
		blockchain.WithMinSuccessPingCount(p.minSuccessRate*p.pingCount/100),
		blockchain.WithIpfsClusterAPI(p.ipfsClusterApi),
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

func (p *Blox) storeCidIPFS(ctx context.Context, c path.Path) error {
	getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if p.rpc == nil {
		return fmt.Errorf("IPFS rpc is undefined")
	}
	_, err := p.rpc.Block().Get(getCtx, c)
	if err != nil {
		log.Errorw("It seems that the link is not found", "c", c, "err", err)
		return err
	}
	pinCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = p.rpc.Pin().Add(pinCtx, c)
	if err != nil {
		log.Errorw("It seems that the link is found but not pinned", "c", c, "err", err)
		return err
	}
	return nil
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
	cidLink := l.(cidlink.Link).Cid
	cidPath := path.FromCid(cidLink)
	// Iterate over the providers and ping
	for _, provider := range providers {
		if provider.ID != p.h.ID() {
			//Found a storer, now pull the cid
			//TODO: Ideally we should have a mechanism to reserve the pull requests and keep the pulled+requests to a max of replication factor
			log.Debugw("Found a storer", "id", provider.ID)

			replicas := len(providers)
			log.Debugw("Checking replicas vs limit", "replicas", replicas, "limit", limit)
			if replicas <= limit {
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
					log.Errorw("Error happened in pulling from provider", "l", l, "err", err)
					err := p.storeCidIPFS(ctx, cidPath)
					if err != nil {
						continue
					}
				}
				statCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel() // Ensures resources are cleaned up after the Stat call
				var stat iface.BlockStat
				if p.rpc != nil {
					stat, err = p.rpc.Block().Stat(statCtx, cidPath)
					if err != nil {
						log.Errorw("It seems that the link is not stored", "l", l, "err", err)
						continue
					}
				}
				p.ex.IpniNotifyLink(l)
				log.Debugw("link might be successfully stored", "l", l, "from", provider.ID, "size", stat.Size())
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

func (p *Blox) GetLastCheckedTime() (time.Time, error) {
	lastCheckedFile := "/internal/.last_time_ipfs_checked"
	info, err := os.Stat(lastCheckedFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, return zero time
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// IpfsClusterPins streams pins from the IPFS Cluster API and sends them through a channel.
func IpfsClusterPins(ctx context.Context, lastChecked time.Time, account string) (<-chan datamodel.Link, <-chan error) {
	pins := make(chan datamodel.Link)
	errChan := make(chan error, 1) // Buffered channel for error

	go func() {
		defer close(pins)
		defer close(errChan)

		client := &http.Client{}
		req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:9094/pins", nil)
		if err != nil {
			errChan <- err
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)

		for decoder.More() {
			var pin api.GlobalPinInfo
			if err := decoder.Decode(&pin); err != nil {
				errChan <- err
				return
			}
			if pin.Created.After(lastChecked) && pin.PeerMap[account].Status == api.TrackerStatusPinned {
				pins <- cidlink.Link{Cid: pin.Cid.Cid}
			}
		}
	}()

	return pins, errChan
}

// This method fetches the pinned items in ipfs-cluster since the lastChecked time
func (p *Blox) ListModifiedStoredLinks(ctx context.Context, lastChecked time.Time, account string) ([]datamodel.Link, error) {
	log.Debugw("ListModifiedStoredLinks", "lastChecked", lastChecked, "account", account)
	var storedLinks []datamodel.Link

	pins, errChan := IpfsClusterPins(ctx, lastChecked, account)

	// Collect pins from the channel until it's closed
	for pin := range pins {
		storedLinks = append(storedLinks, pin)
	}

	// Check for errors after collecting pins
	if err, ok := <-errChan; ok {
		log.Errorw("Error fetching pins", "error", err)
		return nil, err // Return or handle the error as needed
	}
	log.Debugw("ListModifiedStoredLinks result", "storedLinks", storedLinks)

	return storedLinks, nil
}

func hexStringToCID(hexDigest string) (cid.Cid, error) {
	// Decode the hex digest into bytes
	bytes, err := hex.DecodeString(hexDigest)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("error decoding hex digest: %w", err)
	}

	// Construct the multihash using the bytes
	// Assuming the first byte is the multihash code for BLAKE3 (0x1e) and the second byte is the length
	mh, err := multihash.Encode(bytes[2:], uint64(bytes[0]))
	if err != nil {
		return cid.Cid{}, fmt.Errorf("error constructing multihash: %w", err)
	}

	// Construct the CID using dag-cbor codec (0x71) and the constructed multihash
	c := cid.NewCidV1(cid.DagCBOR, mh)

	return c, nil
}
func findDigestHexFromBase32Multibase(base32MultibaseDigest string) (string, error) {
	// Decode the Base32 Multibase digest to get the raw hash bytes
	_, decodedBytes, err := multibase.Decode(base32MultibaseDigest)
	if err != nil {
		return "", fmt.Errorf("error decoding multibase: %w", err)
	}

	// Convert the decoded bytes to a hexadecimal string
	digestHex := hex.EncodeToString(decodedBytes)

	return digestHex, nil
}

func FindCIDFromDigest(base32MultibaseDigest string) (cid.Cid, error) {
	// Decode the Multibase prefix and data
	hexStr, err := findDigestHexFromBase32Multibase(base32MultibaseDigest)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("error finding hex value: %w", err)
	}
	c, err := hexStringToCID(hexStr)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("error finding cid value from hex: %w", err)
	}

	return c, nil
}

// ListModifiedStoredBlocks lists only the folders that have been modified after the last check time
// and returns the filenames of the files created after the last check time in those folders.
func (p *Blox) ListModifiedStoredBlocks(lastChecked time.Time) ([]datamodel.Link, error) {
	log.Debugw("ListModifiedStoredBlocks", "lastChecked", lastChecked)
	blocksDir := "/uniondrive/ipfs_datastore/blocks"
	var modifiedDirs []string
	var modifiedLinks []datamodel.Link
	//lint:ignore S1021 err may be reused for different blocks of logic
	var err error

	// Find directories modified after lastChecked
	err = filepath.WalkDir(blocksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path != blocksDir && d.IsDir() && !strings.HasSuffix(d.Name(), ".temp") {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.ModTime().After(lastChecked) {
				modifiedDirs = append(modifiedDirs, path)
				return filepath.SkipDir
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	log.Debugw("ListModifiedStoredBlocks", "modifiedDirs", modifiedDirs)

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	// Use a channel to safely collect files from multiple goroutines
	linksChan := make(chan datamodel.Link, 1024) // Buffered channel

	// Create a goroutine for each modified directory
	for _, dir := range modifiedDirs {
		wg.Add(1) // Increment the WaitGroup counter
		go func(dir string) {
			defer wg.Done() // Decrement the counter when the goroutine completes
			// Use filepath.WalkDir for more efficient directory traversal
			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// Since WalkDir gives us a DirEntry, we can use its methods instead of os.Stat
				if !d.IsDir() && !strings.HasSuffix(d.Name(), ".temp") && strings.HasSuffix(d.Name(), ".data") {
					info, err := d.Info() // This is only needed if we want to check modification times
					if err != nil {
						return err
					}
					if info.ModTime().After(lastChecked) {
						c, err := p.GetCidv1FromBlockFilename(path)
						if err == nil {
							linksChan <- cidlink.Link{Cid: c}
						}
					}
				}
				return nil
			})
			if err != nil {
				// Handle the error. Since this is in a goroutine, consider using a channel to report errors.
				log.Errorf("Error walking through directory %s: %v", dir, err)
			}
		}(dir)

	}

	// Start a goroutine to close the channel once all goroutines have finished
	go func() {
		wg.Wait()
		close(linksChan)
	}()

	// Collect the results from the channel
	for l := range linksChan {
		modifiedLinks = append(modifiedLinks, l)
	}
	log.Debugw("ListModifiedStoredBlocks", "modifiedLinks", modifiedLinks)

	return modifiedLinks, nil
}

// GetCidv1FromBlockFilename extracts CIDv1 from block filename
func (p *Blox) GetCidv1FromBlockFilename(filename string) (cid.Cid, error) {
	// Implement the logic to extract CIDv1 from filename
	// For example, you can use regular expressions or string manipulation
	// This is just a placeholder implementation
	// Adjust it according to your actual filename format
	// Here's a sample implementation:
	base := filepath.Base(filename)
	b58 := "B" + strings.ToUpper(strings.TrimSuffix(base, filepath.Ext(base)))
	return FindCIDFromDigest(b58)
}

// UpdateLastCheckedTime updates the last checked time
func (p *Blox) UpdateLastCheckedTime() error {
	lastCheckedFile := "/internal/.last_time_ipfs_checked"
	err := os.WriteFile(lastCheckedFile, []byte(time.Now().Format(time.RFC3339)), 0644)
	if err != nil {
		return err
	}
	return nil
}

// UpdateFailedCids updates the last checked time by appending failed CIDs to a file.
// It collects all errors encountered and returns them together.
func (p *Blox) UpdateFailedCids(links []datamodel.Link) error {
	lastFailedFile := "/uniondrive/failed_cids.info"
	// Open the file in append mode. If it doesn't exist, create it.
	file, err := os.OpenFile(lastFailedFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	var errorMessages []string

	for _, link := range links {
		_, err := file.WriteString(link.String() + "\n")
		if err != nil {
			// Collect the error message.
			errorMessages = append(errorMessages, err.Error())
			continue
		}
	}

	if len(errorMessages) > 0 {
		// Join all error messages into a single string and return it as an error.
		return fmt.Errorf("errors occurred while updating failed CIDs: %s", strings.Join(errorMessages, "; "))
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
	var nodeAccount string
	accountFilePath := "/internal/.secrets/account.txt"
	data, err := os.ReadFile(accountFilePath)
	if err != nil {
		log.Errorw("Error reading file", "err", err)
	} else {
		nodeAccount = string(data)
	}

	if p.topicName == "0" {
		// First read account from /internal/.secrets/account.txt
		if nodeAccount != "" {
			found := false

			// Initial delay in seconds
			delay := time.Duration(60) * time.Second
			// Maximum number of retries
			maxRetries := 5
			// Attempt counter
			attempt := 0

			for {
				attempt++
				poolList, err := p.bl.HandlePoolList(ctx)
				if err != nil {
					log.Errorw("Error getting pool list", "err", err, "attempt", attempt)
					if attempt >= maxRetries {
						// Handle the case when max retries are reached, maybe log or break
						log.Errorw("Max retries reached. Giving up.", "maxRetries", maxRetries)
						break
					}
					// Wait for the current delay before retrying
					time.Sleep(delay)
					// Increase the delay for the next attempt, e.g., double it
					delay *= 2
					continue
				}

				//Iterate through the pool list if no error
				for _, pool := range poolList.Pools {
					if found {
						break
					}
					for _, participant := range pool.Participants {
						if found {
							break
						}
						if participant == nodeAccount {
							topicStr := strconv.Itoa(pool.PoolID)
							p.updatePoolName(topicStr)
							p.topicName = topicStr
							found = true
							break
						}
					}
				}
				if found || err == nil {
					break
				}
			}
		}
	}

	if err := p.bl.FetchUsersAndPopulateSets(ctx, p.topicName, true, 15*time.Second); err != nil {
		log.Errorw("FetchUsersAndPopulateSets failed", "err", err)
	}

	// Create an HTTP server instance
	if p.DefaultIPFShttpServer == "fula" {
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
	}

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

	if !p.poolHostMode {
		// Starting a new goroutine for periodic task
		log.Debug("Called wg.Add in blox.start")
		p.wg.Add(1)
		go func() {
			log.Debug("Called wg.Done in blox.start")
			defer p.wg.Done()
			defer log.Debug("Start blox blox.start go routine is ending")
			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					// instead of FetchAvailableManifestsAndStore See what are the new manifests from ther last time we checked in blockstore
					// A method that checks the last time we checked for stored blocks file
					// then it checks the stored files under /uniondrive/ipfs_datastore/blocks/{folders that are not .temp}
					// then for those folders that the change date/time is after the time we last checked it reads the files names that are created after the last time we checked
					// thne it passes each filename to a method called GetCidv1FromBlockFilename(string filename) (string, error) and receives the cidv1 of each file and put all in an array []string
					// Then it calls the method:_, err := p.bl.HandleManifestBatchStore(ctx, p.topicName, storedCids)
					// If no error, it stores the time/date in a file under /internal/.last_time_ipfs_checked so that it can use it in the next run
					// ListStoredBlocks, and GetCidv1FromBlockFilename
					lastCheckedTime, err := p.GetLastCheckedTime()
					if err != nil {
						log.Errorf("Error retrieving last checked time: %v", err)
						continue
					}
					p.topicName = p.getPoolName()
					if p.topicName != "0" {
						shortCtx, shortCtxCancel := context.WithDeadline(p.ctx, time.Now().Add(60*time.Second))
						defer shortCtxCancel()
						p.rpc.Request("repo/gc").Send(shortCtx)
						storedLinks, err := p.ListModifiedStoredBlocks(lastCheckedTime)
						if err != nil {
							log.Errorf("Error listing stored blocks: %v", err)
							continue
						}

						// Call HandleManifestBatchStore method
						if len(storedLinks) > 0 {
							_, err = p.bl.HandleManifestBatchStore(context.TODO(), p.topicName, storedLinks)
							if strings.Contains(err.Error(), "AccountAlreadyStorer") {
								// Log the occurrence of the specific error but do not continue
								log.Warnw("Attempt to store with an account that is already a storer", "err", err, "p.topicName", p.topicName, "storedLinks", storedLinks)
							} else {
								// For any other error, log and continue
								log.Errorw("Error calling HandleManifestBatchStore", "err", err, "p.topicName", p.topicName, "storedLinks", storedLinks)
								p.UpdateFailedCids(storedLinks)
								//continue
							}
						}
					}

					// Update the last checked time
					err = p.UpdateLastCheckedTime()
					if err != nil {
						log.Errorf("Error updating last checked time: %v", err)
					}
				case <-ctx.Done():
					// This will handle the case where the parent context is canceled
					log.Info("Stopping periodic blox.start due to context cancellation")
					return
				case <-p.ctx.Done():
					// This will handle the case where the parent context is canceled
					log.Info("Stopping periodic blox.start due to context cancellation")
					return
				}
			}
		}()
	}

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

func (p *Blox) GetIPFSRPC() *rpc.HttpApi {
	//This is for unit testing and no need to call directly
	return p.rpc
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

	// Use a separate context for shutdown with a timeout
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelShutdown()

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
	if p.IPFShttpServer != nil {
		if IPFSErr := p.IPFShttpServer.Shutdown(ctx); IPFSErr != nil {
			log.Errorw("Error shutting down IPFS HTTP server", "IPFSErr", IPFSErr)
		}
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
	case <-shutdownCtx.Done():
		err := shutdownCtx.Err()
		log.Infow("Shutdown completed with timeout", "err", err)
	}

	// Check for errors from shutdownCtx
	if err := shutdownCtx.Err(); err == context.DeadlineExceeded {
		// Handle the case where shutdown did not complete in time
		log.Warn("Shutdown did not complete in the allotted time")
		return err
	}

	return nil
}
