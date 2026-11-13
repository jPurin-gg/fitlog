package httpapi

import (
	"net/http"
	"net/netip"
	"strings"
)

// WithTrustedProxies configures the proxy addresses allowed to append to
// X-Forwarded-For. Configure this once, before serving requests. No proxy is
// trusted by default; Forwarded and X-Real-IP are never used.
func (h *Handler) WithTrustedProxies(prefixes []netip.Prefix) *Handler {
	h.trustedProxies = append([]netip.Prefix(nil), prefixes...)
	return h
}

func (h *Handler) loginSource(r *http.Request) string {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		// Unknown peers share one bucket rather than using attacker-chosen text.
		return "unknown"
	}
	address := peer.Addr().WithZone("").Unmap()
	if !h.trustsProxy(address) {
		return address.String()
	}

	forwarded := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	// Invalid or excessively long chains fall back to the actual peer. Validate
	// the entire chain before using it so malformed prefixes cannot change keys.
	if len(forwarded) > 32 {
		return address.String()
	}
	addresses := make([]netip.Addr, len(forwarded))
	for index, value := range forwarded {
		ip, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil || ip.Zone() != "" {
			return address.String()
		}
		addresses[index] = ip.Unmap()
	}
	for index := len(addresses) - 1; index >= 0; index-- {
		address = addresses[index]
		if !h.trustsProxy(address) {
			break
		}
	}
	return address.String()
}

func (h *Handler) trustsProxy(address netip.Addr) bool {
	for _, prefix := range h.trustedProxies {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
