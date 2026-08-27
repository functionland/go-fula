package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORS(t *testing.T) {
	called := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})
	h := withCORS(next)

	cases := []struct {
		name       string
		method     string
		path       string
		origin     string
		sfs        string // Sec-Fetch-Site
		wantStatus int
		wantACAO   string
		wantNext   bool
	}{
		{"no origin GET passes through (mobile app / curl)", http.MethodGet, "/properties", "", "", http.StatusOK, "", true},
		{"no origin POST passes through", http.MethodPost, "/peer/exchange", "", "", http.StatusOK, "", true},
		{"no origin OPTIONS passes through untouched (not a CORS preflight)", http.MethodOptions, "/properties", "", "", http.StatusOK, "", true},
		{"no origin GET /ap/disable passes through (native client)", http.MethodGet, "/ap/disable", "", "", http.StatusOK, "", true},
		{"allow-listed origin GET gets ACAO", http.MethodGet, "/properties", "https://blox.fx.land", "cross-site", http.StatusOK, "https://blox.fx.land", true},
		{"staging origin POST gets ACAO", http.MethodPost, "/wifi/connect", "https://functionland.github.io", "cross-site", http.StatusOK, "https://functionland.github.io", true},
		{"docs.fx.land staging origin (org Pages custom domain) allowed", http.MethodPost, "/wifi/connect", "https://docs.fx.land", "cross-site", http.StatusOK, "https://docs.fx.land", true},
		{"localhost dev origin allowed", http.MethodPost, "/wifi/connect", "http://localhost:5173", "cross-site", http.StatusOK, "http://localhost:5173", true},
		{"127.0.0.1 dev origin allowed", http.MethodGet, "/properties", "http://127.0.0.1:4173", "cross-site", http.StatusOK, "http://127.0.0.1:4173", true},
		{"allow-listed origin may hit the side-effecting GET routes", http.MethodGet, "/ap/disable", "https://blox.fx.land", "cross-site", http.StatusOK, "https://blox.fx.land", true},
		{"preflight from allow-listed origin is 204 and never reaches the mux", http.MethodOptions, "/properties", "https://blox.fx.land", "cross-site", http.StatusNoContent, "https://blox.fx.land", false},
		{"preflight from unknown origin is 403", http.MethodOptions, "/properties", "https://evil.example", "cross-site", http.StatusForbidden, "", false},
		{"cross-site POST from unknown origin is 403 (Origin guard)", http.MethodPost, "/wifi/connect", "https://evil.example", "cross-site", http.StatusForbidden, "", false},
		{"cross-site GET from unknown origin passes but gets no ACAO (unreadable by the browser)", http.MethodGet, "/properties", "https://evil.example", "cross-site", http.StatusOK, "", true},
		{"cross-site <img> GET to /ap/disable (no Origin, Sec-Fetch-Site: cross-site) is 403", http.MethodGet, "/ap/disable", "", "cross-site", http.StatusForbidden, "", false},
		{"cross-site <img> GET to /ap/enable is 403", http.MethodGet, "/ap/enable", "", "cross-site", http.StatusForbidden, "", false},
		{"cross-site <img> GET to a read-only route passes (nothing to protect)", http.MethodGet, "/properties", "", "cross-site", http.StatusOK, "", true},
		{"same-origin browser fetch (Sec-Fetch-Site: same-origin) passes", http.MethodGet, "/ap/disable", "", "same-origin", http.StatusOK, "", true},
		{"user-typed navigation (Sec-Fetch-Site: none) passes", http.MethodGet, "/ap/disable", "", "none", http.StatusOK, "", true},
		{"http scheme for the production host is not allowed", http.MethodPost, "/wifi/connect", "http://blox.fx.land", "cross-site", http.StatusForbidden, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = 0
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.sfs != "" {
				req.Header.Set("Sec-Fetch-Site", tc.sfs)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.wantACAO {
				t.Fatalf("ACAO: got %q want %q", got, tc.wantACAO)
			}
			if (called == 1) != tc.wantNext {
				t.Fatalf("next handler called=%d want %v", called, tc.wantNext)
			}
			if tc.wantACAO != "" && rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" {
				t.Fatalf("missing Allow-Methods")
			}
			// Every response whose content depends on Origin must be Vary: Origin (cache poisoning guard);
			// responses to requests without Origin must not gain the header.
			if tc.origin != "" && rec.Header().Get("Vary") != "Origin" {
				t.Fatalf("missing Vary: Origin on an Origin-dependent response")
			}
			if tc.origin == "" && rec.Header().Get("Vary") != "" {
				t.Fatalf("Vary must not be added for requests without Origin")
			}
		})
	}
}

func TestWithCORSEnvOverride(t *testing.T) {
	t.Setenv("WAP_CORS_ORIGINS", "https://custom.example, https://other.example")
	if !originAllowed("https://custom.example") || !originAllowed("https://other.example") {
		t.Fatal("override origins should be allowed")
	}
	if originAllowed("https://blox.fx.land") {
		t.Fatal("defaults must not apply when WAP_CORS_ORIGINS is set")
	}
	if !originAllowed("http://localhost:3000") {
		t.Fatal("local dev origins stay allowed regardless of the override")
	}
}
