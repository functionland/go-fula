package fulamobile

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	"github.com/ipfs/go-datastore"
	"github.com/ipld/go-ipld-prime"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	libp2pping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

// Note to self; copied from gomobile docs:
// All exported symbols in the package must have types that are supported. Supported types include:
//  * Signed integer and floating point types.
//  * String and boolean types.
//  * Byte slice types.
//    * Note that byte slices are passed by reference and support mutation.
//  * Any function type all of whose parameters and results have supported types.
//    * Functions must return either no results, one result, or two results where the type of the second is the built-in 'error' type.
//  * Any interface type, all of whose exported methods have supported function types.
//  * Any struct type, all of whose exported methods have supported function types and all of whose exported fields have supported types.

var rootDatastoreKey = datastore.NewKey("/")

type Client struct {
	h       host.Host
	ds      datastore.Batching
	ls      ipld.LinkSystem
	ex      exchange.Exchange
	bl      blockchain.Blockchain
	bloxPid peer.ID
	relays  []string

	ipfsDHT       *dht.IpfsDHT   // Standard IPFS DHT for peer discovery fallback
	ipfsDHTReady  chan struct{}   // Closed when DHT bootstrap completes
	ipfsDHTCtx    context.Context
	ipfsDHTCancel context.CancelFunc

	streams map[string]*blockchain.StreamBuffer // Map of active streams
	mu      sync.Mutex                          // Mutex for thread-safe access
}

func NewClient(cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.init(&mc); err != nil {
		return nil, err
	}

	// Initialize the streams map for managing active streaming sessions
	mc.streams = make(map[string]*blockchain.StreamBuffer)

	return &mc, nil
}

// ensureConnected attempts to connect to blox using peerstore addresses (direct + relay),
// and falls back to IPFS DHT peer discovery if the direct attempt fails and DHT is enabled.
func (c *Client) ensureConnected(ctx context.Context) error {
	ctx = network.WithUseTransient(ctx, "fx.mobile")

	// Close stale connections to avoid "dial backoff" from expired relay v2
	// circuits that libp2p still considers "connected".
	// See: mainnet/libp2p-service PingPeerProtocol for the same pattern.
	if c.h.Network().Connectedness(c.bloxPid) != network.Connected {
		_ = c.h.Network().ClosePeer(c.bloxPid)
	}

	peerInfo := c.h.Peerstore().PeerInfo(c.bloxPid)

	// Try direct + relay (peerstore addresses)
	connectErr := c.h.Connect(ctx, peerInfo)
	if connectErr == nil {
		return nil
	}

	// Clean up failed connection state so the DHT retry doesn't hit "dial backoff"
	_ = c.h.Network().ClosePeer(c.bloxPid)

	// If DHT is not available, return the original error
	if c.ipfsDHT == nil {
		return connectErr
	}

	log.Infof("Direct connect failed for peer %s, attempting IPFS DHT peer discovery: %v", c.bloxPid, connectErr)

	// Wait for DHT to be ready (up to 15s)
	readyCtx, readyCancel := context.WithTimeout(ctx, 15*time.Second)
	defer readyCancel()
	select {
	case <-c.ipfsDHTReady:
	case <-readyCtx.Done():
		return fmt.Errorf("direct connect failed: %w; IPFS DHT not ready in time", connectErr)
	}

	// Query DHT for peer addresses (up to 30s)
	findCtx, findCancel := context.WithTimeout(ctx, 30*time.Second)
	defer findCancel()
	addrInfo, findErr := c.ipfsDHT.FindPeer(findCtx, c.bloxPid)
	if findErr != nil {
		return fmt.Errorf("direct connect failed: %w; IPFS DHT FindPeer also failed: %v", connectErr, findErr)
	}

	if len(addrInfo.Addrs) == 0 {
		return fmt.Errorf("direct connect failed: %w; IPFS DHT found peer but no addresses", connectErr)
	}

	log.Infof("IPFS DHT discovered %d addresses for peer %s: %v", len(addrInfo.Addrs), c.bloxPid, addrInfo.Addrs)

	// Add discovered addresses to peerstore and retry
	c.h.Peerstore().AddAddrs(c.bloxPid, addrInfo.Addrs, peerstore.TempAddrTTL)
	retryInfo := c.h.Peerstore().PeerInfo(c.bloxPid)
	retryErr := c.h.Connect(ctx, retryInfo)
	if retryErr != nil {
		return fmt.Errorf("direct connect failed: %w; IPFS DHT retry also failed: %v", connectErr, retryErr)
	}
	return nil
}

// ConnectToBlox attempts to connect to blox via the configured address with
// direct → relay → DHT fallback. This function can be used to check if blox
// is currently accessible.
func (c *Client) ConnectToBlox() error {
	if _, ok := c.ex.(exchange.NoopExchange); ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return c.ensureConnected(ctx)
}

// Ping sends libp2p pings to the blox peer and returns a JSON object with results.
// It first ensures connectivity (direct → relay → DHT fallback), then sends 3 pings.
// Returns JSON: {"success": true/false, "successes": N, "avg_rtt_ms": N, "errors": [...]}
// Success is true if at least one ping succeeded.
func (c *Client) Ping() ([]byte, error) {
	type PingResult struct {
		Success    bool     `json:"success"`
		Successes  int      `json:"successes"`
		AvgRttMs   int64    `json:"avg_rtt_ms"`
		Errors     []string `json:"errors"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Ensure we're connected (direct → relay → DHT)
	if err := c.ensureConnected(ctx); err != nil {
		result := PingResult{
			Success:  false,
			Errors:   []string{fmt.Sprintf("connection failed: %v", err)},
		}
		return json.Marshal(result)
	}

	const pingCount = 3
	var successes int
	var totalRtt time.Duration
	var errs []string

	for i := 0; i < pingCount; i++ {
		pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
		pingCtx = network.WithUseTransient(pingCtx, "fx.mobile.ping")
		result := <-libp2pping.Ping(pingCtx, c.h, c.bloxPid)
		pingCancel()
		if result.Error != nil {
			errs = append(errs, fmt.Sprintf("ping %d: %v", i+1, result.Error))
		} else {
			successes++
			totalRtt += result.RTT
		}
	}

	var avgRtt int64
	if successes > 0 {
		avgRtt = (totalRtt / time.Duration(successes)).Milliseconds()
	}

	res := PingResult{
		Success:   successes > 0,
		Successes: successes,
		AvgRttMs:  avgRtt,
		Errors:    errs,
	}
	return json.Marshal(res)
}

// ID returns the libp2p peer ID of the client.
func (c *Client) ID() string {
	return c.h.ID().String()
}

// Flush guarantees that all values stored locally are synced to the baking local storage.
func (c *Client) Flush() error {
	return c.ds.Sync(context.TODO(), rootDatastoreKey)
}

// SetAuth sets authorization on the given peer ID for the given subject.
func (c *Client) SetAuth(on string, subject string, allow bool) error {
	onp, err := peer.Decode(on)
	if err != nil {
		return err
	}
	subp, err := peer.Decode(subject)
	if err != nil {
		return err
	}
	return c.ex.SetAuth(context.TODO(), onp, subp, allow)
}

// Shutdown closes all resources used by Client.
// After calling this function Client must be discarded.
func (c *Client) Shutdown() error {
	ctx := context.TODO()
	xErr := c.ex.Shutdown(ctx)
	// Shut down IPFS DHT before closing the host
	if c.ipfsDHTCancel != nil {
		c.ipfsDHTCancel()
	}
	var dhtErr error
	if c.ipfsDHT != nil {
		dhtErr = c.ipfsDHT.Close()
	}
	hErr := c.h.Close()
	fErr := c.Flush()
	dsErr := c.ds.Close()
	switch {
	case hErr != nil:
		return hErr
	case dhtErr != nil:
		return dhtErr
	case fErr != nil:
		return fErr
	case dsErr != nil:
		return dsErr
	default:
		return xErr
	}
}
