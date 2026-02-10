package blockchain

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"path"
)

const (
	// ProxyListenAddr is the TCP address where go-fula listens for
	// kubo-forwarded blockchain commands.
	ProxyListenAddr = "127.0.0.1:4020"
)

// StartProxy starts a TCP HTTP server on ProxyListenAddr that accepts
// blockchain commands forwarded by kubo's libp2p stream mounting.
// Requests are authenticated via signed headers (see auth_signed.go).
func (bl *FxBlockchain) StartProxy(ctx context.Context) error {
	listener, err := net.Listen("tcp", ProxyListenAddr)
	if err != nil {
		return err
	}
	bl.proxyServer = &http.Server{Handler: http.HandlerFunc(bl.serveProxy)}
	if bl.wg != nil {
		log.Debug("called wg.Add in blockchain StartProxy")
		bl.wg.Add(1)
	}
	go func() {
		if bl.wg != nil {
			log.Debug("called wg.Done in StartProxy blockchain")
			defer bl.wg.Done()
		}
		defer log.Debug("StartProxy blockchain go routine is ending")
		if err := bl.proxyServer.Serve(listener); err != http.ErrServerClosed {
			log.Errorw("Proxy server stopped erroneously", "err", err)
		}
	}()
	log.Infow("Blockchain proxy server started", "addr", ProxyListenAddr)
	return nil
}

// serveProxy handles incoming requests on the TCP proxy server.
// It verifies signed headers, checks authorization, and dispatches the action.
func (bl *FxBlockchain) serveProxy(w http.ResponseWriter, r *http.Request) {
	from, bodyBytes, err := verifySignedRequest(r)
	if err != nil {
		log.Errorw("proxy: signed request verification failed", "err", err)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	action := path.Base(r.URL.Path)
	if !bl.authorized(from, action) {
		log.Errorw("proxy: rejected unauthorized request", "from", from, "action", action)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	log.Debugw("proxy: action permitted", "action", action, "from", from)

	// Restore the body so dispatch handlers can read it
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	bl.dispatch(from, action, w, r)
}

// ShutdownProxy gracefully shuts down the proxy server.
func (bl *FxBlockchain) ShutdownProxy(ctx context.Context) error {
	if bl.proxyServer != nil {
		return bl.proxyServer.Shutdown(ctx)
	}
	return nil
}
