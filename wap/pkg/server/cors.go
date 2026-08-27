package server

import (
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Browser origins allowed to call the WAP API (FxBlox Web at blox.fx.land, its GitHub Pages staging origin,
// and local development). Override with WAP_CORS_ORIGINS="https://a.example,https://b.example".
var defaultCORSOrigins = []string{
	"https://blox.fx.land",
	"https://docs.fx.land",           // functionland.github.io project pages are served under this custom domain (staging)
	"https://functionland.github.io", // kept in case the org site's custom domain is ever removed
}

var localDevOrigin = regexp.MustCompile(`^http://(localhost|127\.0\.0\.1)(:\d+)?$`)

// Routes that have side effects but can be driven by a GET (historical API shape). A cross-site page cannot read
// their responses, but a plain <img>/<script> fetch would still trigger them and carries NO Origin header —
// Chromium does send Sec-Fetch-Site on every request, so those are guarded on that header instead.
//
// Two distinct reasons a route is listed here:
//   - /ap/enable and /ap/disable enforce GET explicitly.
//   - /pools/* enforce NO method at all and read their parameters with r.FormValue, which happily takes them from
//     the query string. So `GET /pools/join?poolID=…` mutates /internal/config.yaml. Without them in this map the
//     guard below classifies such a request as non-mutating and lets it straight through — i.e. the Origin guard
//     would look like it protects the box while `<img src="http://10.42.0.1:3500/pools/join?poolID=evil">` on any
//     page the owner visits still worked. Keep this map in sync with the handlers in server.go: any route that
//     does not reject non-POST requests and has side effects belongs here.
var mutatingGETPaths = map[string]bool{
	"/ap/enable":    true,
	"/ap/disable":   true,
	"/pools/join":   true,
	"/pools/leave":  true,
	"/pools/cancel": true,
}

func allowedCORSOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("WAP_CORS_ORIGINS"))
	if raw == "" {
		return defaultCORSOrigins
	}
	var out []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func originAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, o := range allowedCORSOrigins() {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return localDevOrigin.MatchString(origin)
}

// withCORS wraps the WAP mux for browser clients:
//   - adds CORS response headers for allow-listed origins;
//   - answers CORS preflights (OPTIONS with an Origin header) itself, since the route handlers reject OPTIONS with 405;
//   - rejects state-changing requests from non-allow-listed browser contexts: any non-GET/HEAD request carrying a
//     non-allow-listed Origin (cross-site form POST), and the side-effecting GET routes when the browser reports
//     Sec-Fetch-Site other than same-origin/none (cross-site <img>/<script> fetches, which carry no Origin).
//
// Requests without Origin or Sec-Fetch-Site headers — the mobile app, curl, the on-device BLE proxy — are passed
// through untouched (including a bare OPTIONS). Responses that depend on Origin carry `Vary: Origin`.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		sfs := strings.ToLower(r.Header.Get("Sec-Fetch-Site"))
		crossSiteFetch := sfs != "" && sfs != "same-origin" && sfs != "none"
		if origin == "" && !crossSiteFetch {
			next.ServeHTTP(w, r)
			return
		}

		allowed := originAllowed(origin)
		if origin != "" {
			h := w.Header()
			h.Add("Vary", "Origin")
			if allowed {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "content-type")
				h.Set("Access-Control-Max-Age", "600")
				// Private Network Access. The whole point of this server is to be reached at 10.42.0.1 from a
				// page on the public internet (https://blox.fx.land), which is exactly the cross-address-space
				// request Chrome gates. When the browser asserts a local/private target it sends this preflight
				// header, and WITHOUT the matching allow header the fetch fails outright — not with a CORS error
				// the app can explain, but a bare "TypeError: Failed to fetch". Observed on real hardware.
				if strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
					h.Set("Access-Control-Allow-Private-Network", "true")
				}
			}
			if r.Method == http.MethodOptions {
				if allowed {
					w.WriteHeader(http.StatusNoContent)
				} else {
					http.Error(w, "origin not allowed", http.StatusForbidden)
				}
				return
			}
		}

		mutating := (r.Method != http.MethodGet && r.Method != http.MethodHead) || mutatingGETPaths[r.URL.Path]
		if mutating && !allowed {
			// Logged because in the field this is indistinguishable from "the box is broken": the web app just
			// sees 403. A wrong WAP_CORS_ORIGINS, or a client that unexpectedly sends an Origin, shows up here.
			log.Warnw("rejected a state-changing request from a non-allow-listed browser context",
				"path", r.URL.Path, "method", r.Method, "origin", origin, "secFetchSite", sfs)
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
