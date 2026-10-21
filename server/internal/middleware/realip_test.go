package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedRealIP(t *testing.T) {
	mw := TrustedRealIP(ParsePrefixes([]string{"10.0.0.0/8", "127.0.0.1"}))
	var got string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))

	cases := []struct {
		name, remote, xff, realIP, want string
	}{
		{"untrusted peer cannot spoof", "203.0.113.9:4000", "10.9.9.9", "", "203.0.113.9:4000"},
		{"trusted proxy forwards client", "10.0.0.5:4000", "198.51.100.7", "", "198.51.100.7"},
		{"client-supplied hops are skipped", "10.0.0.5:4000", "6.6.6.6, 198.51.100.7, 10.0.0.9", "", "198.51.100.7"},
		{"x-real-ip from proxy", "127.0.0.1:4000", "", "198.51.100.8", "198.51.100.8"},
		{"garbage keeps socket address", "10.0.0.5:4000", "not-an-ip", "", "10.0.0.5:4000"},
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if tc.realIP != "" {
			r.Header.Set("X-Real-IP", tc.realIP)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
