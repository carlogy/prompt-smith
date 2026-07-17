package server

import (
	"net"
	"net/http"
	"net/url"
)

// allowedHosts is the set of hostnames (port-independent - see
// enforceLocalOnly) this server accepts requests for.
var allowedHosts = map[string]bool{
	"127.0.0.1": true,
	"localhost": true,
	"::1":       true,
}

// enforceLocalOnly wraps next with two checks that together block a
// malicious web page from reaching this loopback-only server through
// the user's own browser:
//
//   - Host: browsers set this faithfully (it's a forbidden header a
//     page's script can't override) to the request's actual
//     destination authority, so restricting it to known loopback
//     hostnames defeats DNS rebinding - a domain that resolves to
//     127.0.0.1 still sends its own Host, never "127.0.0.1". The port
//     isn't checked: TCP already only delivers a request to the port
//     this server is actually listening on, so pinning it here would
//     add nothing against a browser, which can't spoof Host anyway.
//   - Origin: sent by fetch/XHR (same-origin and cross-origin alike)
//     but never present on a plain top-level GET navigation, so a
//     missing Origin is allowed - that's how the page itself loads;
//     a present-but-mismatched Origin means some other page's script
//     made this request, and is rejected.
func enforceLocalOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !allowedHosts[host] {
			writeJSONError(w, http.StatusForbidden, "this server only accepts requests addressed to localhost")
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" && !allowedHosts[originHostname(origin)] {
			writeJSONError(w, http.StatusForbidden, "cross-origin requests are not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// originHostname extracts the hostname from an Origin header value
// (e.g. "http://127.0.0.1:54321" -> "127.0.0.1"). An opaque origin
// (the literal string "null", sent by e.g. a sandboxed iframe) or
// anything else that doesn't parse into a URL with a host falls back
// to the raw value, which will simply never match allowedHosts - the
// safe default is to reject, not to guess.
func originHostname(origin string) string {
	if u, err := url.Parse(origin); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return origin
}
