package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForwardedTLS(t *testing.T) {
	tests := []struct {
		name       string
		trust      bool
		header     string
		wantTLSSet bool
	}{
		{"trust+https sets TLS", true, "https", true},
		{"trust+chain https first", true, "https, http", true},
		{"trust+http leaves nil", true, "http", false},
		{"trust+missing leaves nil", true, "", false},
		{"no trust ignores header", false, "https", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotTLSSet bool
			h := forwardedTLS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotTLSSet = r.TLS != nil
			}), tc.trust)

			req := httptest.NewRequest(http.MethodPost, "http://example.test/acme/new-account", nil)
			req.TLS = nil
			if tc.header != "" {
				req.Header.Set("X-Forwarded-Proto", tc.header)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			if gotTLSSet != tc.wantTLSSet {
				t.Errorf("r.TLS set = %v, want %v", gotTLSSet, tc.wantTLSSet)
			}
		})
	}
}
