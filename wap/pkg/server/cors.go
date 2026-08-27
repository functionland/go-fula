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
//   - answers OPTIONS preflights itself (route handlers reject OPTIONS with 405);
//   - rejects state-changing requests that carry a NON-allow-listed Origin (a cross-site form POST from any
//     page a user visits while on the FxBlox hotspot). Browsers always send Origin on cross-origin POSTs.
//
// Requests without an Origin header — the mobile app, curl, the on-device BLE proxy — are passed through untouched.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := originAllowed(origin)
		if allowed {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "content-type")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			if allowed {
				w.WriteHeader(http.StatusNoContent)
			} else {
				http.Error(w, "origin not allowed", http.StatusForbidden)
			}
			return
		}
		if origin != "" && !allowed && r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
