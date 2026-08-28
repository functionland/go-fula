package server

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	"gopkg.in/yaml.v3"
)

// Setup over the LAN, for as long as the Blox has no owner.
//
// The WAP API is normally reachable only on the hotspot (10.42.0.1) and loopback. That works for the phone,
// which can join the FxBlox Wi-Fi, but it is painful from a desktop browser: joining the hotspot costs the
// machine its internet connection, and the box has no owner yet so there is nothing to protect. Binding the
// LAN as well while the box is unconfigured makes first-time setup work from an ordinary computer.
//
// Two properties keep this from becoming a hole:
//
//  1. The LAN listeners are only created when the box has no authorizer.
//  2. Every request they serve is re-checked against the CURRENT config (`lanSetupGuard`). `/peer/exchange`
//     can be called more than once, so a listener left open after setup would let anyone on the network
//     re-claim the box. Re-checking means the window closes the instant an owner is set, without needing to
//     tear the listener down.
//
// The trust model while unconfigured is the same one the open FxBlox hotspot already implies — first
// claimant wins — just reachable from the wired network too. It is deliberately NOT extended past setup.

// bloxHasOwner reports whether an authorizer has been set in the fula config.
//
// A missing or unreadable config is treated as "no owner": that is the genuine state of a fresh box (the file
// is written during setup), and failing the other way would make first-time setup impossible over the LAN.
func bloxHasOwner() bool {
	data, err := os.ReadFile(config.FULA_CONFIG_PATH)
	if err != nil {
		return false
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false
	}
	return strings.TrimSpace(cfg.Authorizer) != ""
}

// lanSetupGuard serves only while the box is unowned. Applied to the LAN listeners, never to the hotspot or
// loopback ones, so it cannot affect the phone or the on-box scripts.
func lanSetupGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bloxHasOwner() {
			log.Infow("refused a LAN setup request: this Blox already has an owner",
				"path", r.URL.Path, "remote", r.RemoteAddr)
			http.Error(w, "this Blox is already set up; manage it over the hotspot or Bluetooth", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// skipLANInterface filters out anything that is not a real local network interface: loopback, the hotspot
// itself (already bound), and Docker's bridges and veth pairs, which are RFC1918 but pointless to serve on.
func skipLANInterface(name string) bool {
	if name == "lo" {
		return true
	}
	for _, p := range []string{"docker", "br-", "veth", "dummy", "tun", "tap"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// lanListenAddrs returns "<ip>:<port>" for each private IPv4 address the box holds on a real LAN interface,
// excluding the hotspot address (bound separately) and loopback.
func lanListenAddrs(port string, apIP string) []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Warnw("LAN setup bind: could not list interfaces", "err", err)
		return nil
	}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || skipLANInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() || !ip.IsPrivate() {
				continue
			}
			if ip.String() == apIP {
				continue
			}
			out = append(out, net.JoinHostPort(ip.String(), port))
		}
	}
	return out
}

// How often the background watcher retries the hotspot bind once the startup wait has given up.
var hotspotWatchInterval = 15 * time.Second

// hotspotWatchIntervalForTests shortens the retry cadence so a test does not wait a real interval.
func hotspotWatchIntervalForTests(d time.Duration) func() {
	prev := hotspotWatchInterval
	hotspotWatchInterval = d
	return func() { hotspotWatchInterval = prev }
}

// startHotspotWatch begins watching for the hotspot with a lifetime that ends at mc.Close(), and nowhere else.
//
// It takes no context on purpose. Serve() creates its own `ctx` with `defer cancel()`, so handing that one to a
// background goroutine gives it a context that is already cancelled by the time the goroutine's first tick
// arrives — the watcher would return immediately and the give-up bug would look fixed while doing nothing.
// Owning the lifetime here means no caller can make that mistake.
func startHotspotWatch(mc *multiCloser, mux http.Handler, listenAddr string) {
	mc.mu.Lock()
	if mc.closed {
		mc.mu.Unlock() // shutting down already; never start the goroutine
		return
	}
	if mc.stopWatch == nil {
		mc.stopWatch = make(chan struct{})
	}
	stop := mc.stopWatch
	mc.mu.Unlock()

	watchForHotspot(stop, mc, mux, listenAddr)
}

// watchForHotspot keeps trying to bind the hotspot address until it succeeds or `stop` is closed.
//
// Serve()'s startup wait is bounded (10 minutes), and when it expires the process previously served only
// loopback for the rest of its life. That is the wrong shape for this device: the FxBlox connection has
// autoconnect=no, so the AP is routinely down — restarting fula_go tears it down too — and it may be brought
// up much later by readiness-check, /ap/enable, or a person. A box in that state answered nothing on
// 10.42.0.1:3500, so joining the FxBlox hotspot appeared to fail, and the only cure was another restart which
// dropped the AP again. Observed on hardware, and the reason a user who joined the hotspot got "cannot
// connect" on a perfectly healthy box.
//
// Binding is the readiness test: net.Listen fails with "cannot assign requested address" until the interface
// holds the IP, so no separate interface polling is needed.
func watchForHotspot(stop <-chan struct{}, mc *multiCloser, mux http.Handler, listenAddr string) {
	go func() {
		ticker := time.NewTicker(hotspotWatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			ln, err := net.Listen("tcp", listenAddr)
			if err != nil {
				continue
			}
			if !mc.add(ln) {
				return // shutting down; add() already closed it
			}
			log.Infof("Hotspot appeared later than expected — now also serving %s", listenAddr)
			go func() {
				if serveErr := http.Serve(ln, withCORS(mux)); serveErr != nil && !strings.Contains(serveErr.Error(), "use of closed network connection") {
					log.Errorw("Hotspot server stopped", "err", serveErr)
				}
			}()
			return
		}
	}()
}

// addKuboIdentityState reports whether kubo is still running an identity it is about to replace.
//
// Setting up a Blox that already had an identity — a re-pair after a factory reset — derives a new one and
// writes it into kubo's config, but the running daemon keeps serving the old identity until it restarts.
// A client that finishes setup during that window stores a peer ID it cannot reach: dialling the newly
// configured ID fails with NO_RESERVATION (the relay holds a reservation for the OLD peer), while the live ID
// stops working as soon as kubo restarts. Reproduced on hardware, and the app's own log showed exactly that —
// four dial candidates, all NO_RESERVATION, until kubo was restarted and both IDs agreed.
//
// `kubo_peer_id` keeps its existing meaning (the running daemon) so nothing that reads it today changes.
// These fields are added only when the two genuinely disagree, so a settled box's response is byte-identical
// to before:
//
//	kubo_configured_peer_id  the identity kubo will use after its next restart
//	kubo_identity_pending    true — setup has not fully taken effect yet
//
// A client seeing `kubo_identity_pending` should treat the configured ID as the Blox's real identity and wait
// for the restart before expecting to reach it, rather than recording whichever value it happened to read.
func addKuboIdentityState(out map[string]interface{}, livePeerID string) {
	configured, err := wifi.GetKuboConfiguredPeerID()
	if err != nil || configured == "" || livePeerID == "" || configured == livePeerID {
		return
	}
	out["kubo_configured_peer_id"] = configured
	out["kubo_identity_pending"] = true
	log.Warnw("kubo is running an identity that its config has already replaced; a restart is required",
		"live", livePeerID, "configured", configured)
}

// startAuxListeners brings up every listener that does not depend on the hotspot interface: loopback always,
// plus the LAN addresses while the box has no owner. Called before the hotspot wait loop so a box with its AP
// down still serves 127.0.0.1 immediately — the on-box scripts depend on it.
func startAuxListeners(mc *multiCloser, addrs *[]string, mux http.Handler, port string, apIP string) {
	localhostAddr := "127.0.0.1:" + port
	if ln, err := net.Listen("tcp", localhostAddr); err == nil {
		mc.listeners = append(mc.listeners, ln)
		*addrs = append(*addrs, localhostAddr)
		go func() {
			if serveErr := http.Serve(ln, withCORS(mux)); serveErr != nil && !strings.Contains(serveErr.Error(), "use of closed network connection") {
				log.Errorw("Serve could not initialize on 127.0.0.1", "err", serveErr)
			}
		}()
	} else {
		log.Errorw("Failed to use 127.0.0.1 for serve", "err", err)
	}

	for _, ln := range startLANSetupListeners(mux, port, apIP) {
		mc.listeners = append(mc.listeners, ln)
		*addrs = append(*addrs, ln.Addr().String())
	}
}

// startLANSetupListeners binds the LAN addresses for first-time setup and returns the listeners it opened.
// A bind failure on one address is logged and skipped — this is a convenience path and must never stop the
// hotspot or loopback servers from running.
func startLANSetupListeners(mux http.Handler, port string, apIP string) []net.Listener {
	if bloxHasOwner() {
		return nil
	}
	handler := lanSetupGuard(withCORS(mux))
	var opened []net.Listener
	for _, addr := range lanListenAddrs(port, apIP) {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Warnw("LAN setup bind failed", "addr", addr, "err", err)
			continue
		}
		opened = append(opened, ln)
		log.Infof("Blox has no owner yet — also serving setup on %s", addr)
		go func(l net.Listener) {
			if err := http.Serve(l, handler); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Errorw("LAN setup server stopped", "err", err)
			}
		}(ln)
	}
	return opened
}
