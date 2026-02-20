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

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/common"
	"github.com/ipfs-cluster/ipfs-cluster/api"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/peer"
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
		bl   *blockchain.FxBlockchain
		once sync.Once

		serverKuboPeerID   string
		serverKuboPeerIDMu sync.RWMutex
	}
)

func (p *Blox) getServerKuboPeerID() string {
	p.serverKuboPeerIDMu.RLock()
	defer p.serverKuboPeerIDMu.RUnlock()
	return p.serverKuboPeerID
}

func (p *Blox) setServerKuboPeerID(id string) {
	p.serverKuboPeerIDMu.Lock()
	defer p.serverKuboPeerIDMu.Unlock()
	p.serverKuboPeerID = id
}

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

	p.bl, err = blockchain.NewFxBlockchain(
		blockchain.NewSimpleKeyStorer(p.secretsPath),
		blockchain.WithAuthorizer(opts.authorizer),
		blockchain.WithAuthorizedPeers(opts.authorizedPeers),
		blockchain.WithBlockchainEndPoint(p.blockchainEndpoint),
		blockchain.WithSecretsPath(p.secretsPath),
		blockchain.WithSelfPeerID(p.selfPeerID),
		blockchain.WithTimeout(65),
		blockchain.WithWg(p.wg),
		blockchain.WithFetchFrequency(1*time.Minute),
		blockchain.WithTopicName(p.topicName),
		blockchain.WithChainName(p.chainName),
		blockchain.WithUpdatePoolName(p.updatePoolName),
		blockchain.WithGetPoolName(p.getPoolName),
		blockchain.WithUpdateChainName(p.updateChainName),
		blockchain.WithGetChainName(p.getChainName),
		blockchain.WithRelays(p.relays),
		blockchain.WithMaxPingTime(p.maxPingTime),
		blockchain.WithIpfsClient(p.rpc),
		blockchain.WithMinSuccessPingCount(p.minSuccessRate*p.pingCount/100),
		blockchain.WithIpfsClusterAPI(p.ipfsClusterApi),
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
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
	// Data replication has moved to kubo/IPFS-cluster.
	// StoreCid via exchange pull is no longer supported.
	return errors.New("StoreCid is no longer supported: data replication uses kubo/IPFS-cluster")
}

func (p *Blox) StoreManifest(ctx context.Context, links []blockchain.LinkWithLimit, maxCids int) error {
	// Data replication has moved to kubo/IPFS-cluster.
	return errors.New("StoreManifest is no longer supported: data replication uses kubo/IPFS-cluster")
}

// FetchAvailableManifestsAndStore fetches available manifests and stores them.
func (p *Blox) FetchAvailableManifestsAndStore(ctx context.Context, maxCids int) error {
	// Data replication has moved to kubo/IPFS-cluster.
	return errors.New("FetchAvailableManifestsAndStore is no longer supported: data replication uses kubo/IPFS-cluster")
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
	if err := p.bl.Start(ctx); err != nil {
		return err
	}

	// Wait for kubo and register p2p protocols
	kuboAPI := getKuboAPIAddr(p.kuboAPIAddr)
	if err := waitForKuboAndRegister(ctx, kuboAPI); err != nil {
		log.Errorw("Failed to register kubo protocols", "err", err)
		// Continue startup even if kubo registration fails —
		// the health check loop will retry.
	}

	// Start health check loop to re-register protocols if kubo restarts
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer log.Debug("watchKuboP2P goroutine is ending")
		p.watchKuboP2P(ctx)
	}()

	// Register cluster tunnel forward immediately if pool is already known from config,
	// before potentially blocking on chain discovery for up to ~15 minutes.
	if p.topicName != "0" {
		kuboAPI := getKuboAPIAddr(p.kuboAPIAddr)
		serverKuboID, err := fetchServerKuboPeerID(p.topicName)
		if err != nil {
			log.Warnw("Failed to fetch server kubo peer ID for cluster tunnel", "err", err)
		} else {
			p.setServerKuboPeerID(serverKuboID)
			if err := registerClusterForward(kuboAPI, serverKuboID); err != nil {
				log.Warnw("Failed to register cluster forward", "err", err)
			}
		}
	}

	// Check if we need to discover pool membership (either no pool name or no chain name)
	needsDiscovery := p.topicName == "0" || (p.getChainName != nil && p.getChainName() == "")

	if needsDiscovery {
		found := false
		foundChain := ""

		// Initial delay in seconds
		delay := time.Duration(60) * time.Second
		// Maximum number of retries
		maxRetries := 5
		// Attempt counter
		attempt := 0

		// List of chains to check (skale first as default)
		chainList := []string{"skale", "base"}

		for {
			attempt++
			log.Infow("Starting pool discovery", "attempt", attempt, "PeerID", p.selfPeerID.String())

			// Try each chain
			for _, chainName := range chainList {
				if found {
					break
				}

				log.Debugw("Checking chain for pools", "chain", chainName, "attempt", attempt)

				// Get pool list for this chain
				poolList, err := p.bl.HandleEVMPoolList(ctx, chainName)
				if err != nil {
					log.Errorw("Error getting pool list from chain", "chain", chainName, "err", err, "attempt", attempt)
					continue // Try next chain
				}

				// Check each pool to see if our peerID is a member
				for _, pool := range poolList.Pools {
					if found {
						break
					}

					// peerID format for membership check
					membershipReq := blockchain.IsMemberOfPoolRequest{
						PeerID:    p.selfPeerID.String(),
						PoolID:    pool.ID,
						ChainName: chainName,
					}

					membershipResp, err := p.bl.HandleIsMemberOfPool(ctx, membershipReq)
					if err != nil {
						log.Errorw("Error checking pool membership", "chain", chainName, "poolID", pool.ID, "err", err)
						continue // Try next pool
					}

					if membershipResp.IsMember {
						topicStr := strconv.Itoa(int(pool.ID))
						log.Infow("Found pool membership", "chain", chainName, "poolID", pool.ID, "memberAddress", membershipResp.MemberAddress)

						// Update pool name
						if p.updatePoolName != nil {
							if err := p.updatePoolName(topicStr); err != nil {
								log.Errorw("Failed to update pool name", "poolName", topicStr, "err", err)
							}
						}
						p.topicName = topicStr

						// Update chain name
						if p.updateChainName != nil {
							if err := p.updateChainName(chainName); err != nil {
								log.Errorw("Failed to update chain name", "chainName", chainName, "err", err)
							}
						}
						p.chainName = chainName

						found = true
						foundChain = chainName
						break
					}
				}
			}

			if found {
				log.Infow("Pool discovery completed successfully", "poolName", p.topicName, "chainName", foundChain, "attempt", attempt)
				break
			}

			if attempt >= maxRetries {
				log.Errorw("Max retries reached for pool discovery. Giving up.", "maxRetries", maxRetries)
				break
			}

			// Wait before retrying
			log.Debugw("Pool discovery attempt failed, retrying", "attempt", attempt, "delay", delay)
			time.Sleep(delay)
			// Increase the delay for the next attempt
			delay *= 2
		}

		if !found {
			log.Warnw("Could not find pool membership on any chain", "peerID", p.selfPeerID.String(), "chains", chainList)
		}
	}

	// Register cluster tunnel forward if discovery found a NEW pool (topicName was "0" before).
	// If topicName was already set from config, the forward was registered above before discovery.
	if p.topicName != "0" && p.getServerKuboPeerID() == "" {
		kuboAPI := getKuboAPIAddr(p.kuboAPIAddr)
		serverKuboID, err := fetchServerKuboPeerID(p.topicName)
		if err != nil {
			log.Warnw("Failed to fetch server kubo peer ID for cluster tunnel", "err", err)
		} else {
			p.setServerKuboPeerID(serverKuboID)
			if err := registerClusterForward(kuboAPI, serverKuboID); err != nil {
				log.Warnw("Failed to register cluster forward", "err", err)
			}
		}
	}

	if err := p.bl.FetchUsersAndPopulateSets(ctx, p.topicName, true, 15*time.Second); err != nil {
		log.Errorw("FetchUsersAndPopulateSets failed", "err", err)
	}
	recoverOut := make(chan api.GlobalPinInfo)
	go func() {
		err := p.ipfsClusterApi.RecoverAll(ctx, true, recoverOut)
		if err != nil {
			// Handle error
			log.Errorw("RecoverAll error", "err", err.Error())
		}
	}()

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
							// TODO: Check if the manifests are available to be stored or not
							/*
								This uses old substrate and we need to change to new EVM
								availableLinks, err := p.bl.HandleManifestAvailableAllaccountsBatch(shortCtx, p.topicName, storedLinks)
								if err != nil {
									log.Errorw("Error checking available manifests", "err", err)
									continue // Or handle the error appropriately
								}
								// If available then submit to store
								_, err = p.bl.HandleManifestBatchStore(context.TODO(), p.topicName, availableLinks)
								if err != nil {
									if strings.Contains(err.Error(), "AccountAlreadyStorer") {
										// Log the occurrence of the specific error but do not continue
										log.Warnw("Attempt to store with an account that is already a storer", "err", err, "p.topicName", p.topicName, "availableLinks", availableLinks)
									} else if strings.Contains(err.Error(), "Transaction is outdated") {
										log.Error("Transaction is outdated")
										continue
									} else if strings.Contains(err.Error(), "syncing") {
										log.Error("The chain is syncing")
										continue
									} else {
										// For any other error, log and continue
										log.Errorw("Error calling HandleManifestBatchStore", "err", err, "p.topicName", p.topicName, "availableLinks", availableLinks)
										p.UpdateFailedCids(availableLinks)
										//continue
									}
								}
							*/
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
	return p.bl.SetAuth(ctx, on, subject, allow)
}

func (p *Blox) StartPingServer(ctx context.Context) error {
	return p.bl.StartPingServer(ctx)
}

func (p *Blox) Ping(ctx context.Context, to peer.ID) (int, int, error) {
	// Ping via libp2p is no longer supported; use kubo p2p forwarding.
	return 0, 0, errors.New("direct libp2p ping is no longer supported")
}

func (p *Blox) PingDht(to peer.ID) error {
	// DHT ping via exchange is no longer supported.
	return errors.New("DHT ping is no longer supported")
}

func (p *Blox) GetBlMembers() map[peer.ID]common.MemberStatus {
	return p.bl.GetMembers()
}

func (p *Blox) GetIPFSRPC() *rpc.HttpApi {
	return p.rpc
}

func (p *Blox) BloxFreeSpace(ctx context.Context, to peer.ID) ([]byte, error) {
	return p.bl.BloxFreeSpace(ctx, to)
}

func (p *Blox) EraseBlData(ctx context.Context, to peer.ID) ([]byte, error) {
	return p.bl.EraseBlData(ctx, to)
}

func (p *Blox) StartAnnouncementServer(ctx context.Context) error {
	// Announcements via pubsub are no longer used; pool joins are handled via blockchain API.
	return nil
}

func (p *Blox) AnnounceJoinPoolRequestPeriodically(ctx context.Context) {
	// Announcements via pubsub are no longer used; pool joins are handled via blockchain API.
}

func (p *Blox) ProvideLinkByDht(l ipld.Link) error {
	// DHT operations via exchange are no longer supported.
	return errors.New("DHT provide is no longer supported")
}

func (p *Blox) FindLinkProvidersByDht(l ipld.Link) ([]peer.AddrInfo, error) {
	// DHT operations via exchange are no longer supported.
	return nil, errors.New("DHT find providers is no longer supported")
}

func (p *Blox) UpdateDhtPeers(peers []peer.ID) error {
	// DHT operations via exchange are no longer supported.
	return errors.New("DHT update peers is no longer supported")
}

func (p *Blox) Shutdown(ctx context.Context) error {
	log.Info("Shutdown in progress")

	// Cancel the context to signal all operations to stop
	p.cancel()

	// Use a separate context for shutdown with a timeout
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelShutdown()

	// Shutdown the blockchain component
	if bErr := p.bl.Shutdown(ctx); bErr != nil {
		log.Errorw("Error occurred in blockchain shutdown", "bErr", bErr)
		return bErr
	} else {
		log.Debug("Blockchain shutdown done")
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
