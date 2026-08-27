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
		origin     string
		wantStatus int
		wantACAO   string
		wantNext   bool
	}{
		{"no origin GET passes through (mobile app / curl)", http.MethodGet, "", http.StatusOK, "", true},
		{"no origin POST passes through", http.MethodPost, "", http.StatusOK, "", true},
		{"allow-listed origin GET gets ACAO", http.MethodGet, "https://blox.fx.land", http.StatusOK, "https://blox.fx.land", true},
		{"staging origin POST gets ACAO", http.MethodPost, "https://functionland.github.io", http.StatusOK, "https://functionland.github.io", true},
		{"localhost dev origin allowed", http.MethodPost, "http://localhost:5173", http.StatusOK, "http://localhost:5173", true},
		{"127.0.0.1 dev origin allowed", http.MethodGet, "http://127.0.0.1:4173", http.StatusOK, "http://127.0.0.1:4173", true},
		{"preflight from allow-listed origin is 204 and never reaches the mux", http.MethodOptions, "https://blox.fx.land", http.StatusNoContent, "https://blox.fx.land", false},
		{"preflight from unknown origin is 403", http.MethodOptions, "https://evil.example", http.StatusForbidden, "", false},
		{"cross-site POST from unknown origin is 403 (Origin guard)", http.MethodPost, "https://evil.example", http.StatusForbidden, "", false},
		{"cross-site GET from unknown origin passes but gets no ACAO (unreadable by the browser)", http.MethodGet, "https://evil.example", http.StatusOK, "", true},
		{"http scheme for the production host is not allowed", http.MethodPost, "http://blox.fx.land", http.StatusForbidden, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = 0
			req := httptest.NewRequest(tc.method, "/properties", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
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
			if tc.wantACAO != "" {
				if rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" {
					t.Fatalf("missing Allow-Methods")
				}
				if rec.Header().Get("Vary") != "Origin" {
					t.Fatalf("missing Vary: Origin")
				}
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
