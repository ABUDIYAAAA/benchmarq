package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedRealIP sets RemoteAddr to the client address reported by a reverse
// proxy, but only when the request actually came from one of the trusted
// proxy ranges. Headers from anyone else are ignored, so clients cannot
// spoof their address (which IP allowlists and session logs rely on).
//
// X-Forwarded-For is read right to left, skipping trusted proxies, and the
// first untrusted hop is taken as the client.
func TrustedRealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		a = a.Unmap()
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if peer, ok := remoteAddr(r.RemoteAddr); ok && isTrusted(peer) {
				if client, ok := forwardedClient(r, isTrusted); ok {
					r.RemoteAddr = client.String()
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func forwardedClient(r *http.Request, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
		hops := strings.Split(strings.Join(xff, ","), ",")
		for i := len(hops) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
			if err != nil {
				return netip.Addr{}, false
			}
			if !isTrusted(a) || i == 0 {
				return a.Unmap(), true
			}
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		if a, err := netip.ParseAddr(real); err == nil {
			return a.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

func remoteAddr(addr string) (netip.Addr, bool) {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	a, err := netip.ParseAddr(host)
	return a, err == nil
}

// ParsePrefixes parses CIDR ranges and bare addresses, skipping invalid ones.
func ParsePrefixes(list []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(list))
	for _, s := range list {
		s = strings.TrimSpace(s)
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p.Masked())
		} else if a, err := netip.ParseAddr(s); err == nil {
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
		}
	}
	return out
}
