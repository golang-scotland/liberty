package middleware

import (
	"net"
	"net/http"
	"strings"

	"golang.scot/liberty/env"

	"github.com/gnanderson/trie"
)

// ApiWhitelist instructs an HTTP API handler to make this open to any remote
// IP. This can be modified to being a 'greylist' entry by specificing a list
// of IP's in CIDR format which the resource is accessible to.
//
// A whitelist IP entry for a path will not override any previous IP restrictions
// in the handler chain. If the root entry in the proxy list has a list of IP
// restrictions, and this whitelist has a list of IP entries, then only the IPs
// in this list which also have a previous entry in the handler chain will be
// allowed access.
type ApiWhitelist struct {
	Path      string   `yaml:"path"`
	IPs       []string `yaml:"ips"`
	Hostnames []string `yaml:"hostnames"`
}

// an apiWhitelist is a slice of allowed ip nets and/or a slice of remote hostnames
type whitelistEntry struct {
	nets  []*net.IPNet
	hosts []string
}

func (a *whitelistEntry) hasIP(ip net.IP) bool {
	for _, ipNet := range a.nets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
func (a *whitelistEntry) hasHost(ip net.IP) bool {
	if len(a.hosts) == 0 {
		return false
	}
	names, err := net.LookupAddr(ip.String())
	if err != nil {
		return false
	}

	for _, host := range a.hosts {
		for _, name := range names {
			if strings.HasSuffix(name, host) {
				return true
			}
		}
	}
	return false
}

func (a *whitelistEntry) allows(ip net.IP) bool {
	return a.hasIP(ip) || a.hasHost(ip)
}

// ApiHandler further restricts access to the API. Only certain endpoints are
// open by default, and even open paths can have further IP or hostname based
// restrictions. This is the second layer of the onion, the first potentially
// being an IPRestrictedHandler
type ApiHandler struct {
	whitelist *trie.Trie
}

// NewApiHandler builds a gated whitelist access handler
func NewApiHandler(whitelist []*ApiWhitelist) *ApiHandler {
	tr := &trie.Trie{}
	for _, wl := range whitelist {
		nets := ips2nets(wl.IPs)
		awl := &whitelistEntry{nets, wl.Hostnames}
		tr.Put(wl.Path, awl)
	}

	return &ApiHandler{whitelist: tr}
}

func (ah *ApiHandler) Chain(h http.Handler) http.Handler {
	appEnv := env.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if url path is a sub path of a whitelisted prefex e.g. the path is
		// /api/foo/bar and the whitelist contains /api/foo then this will be
		// allowed. Similarly we will proceed if the path and registered path
		// prefix match exactly.
		if key := ah.whitelist.LongestPrefix(r.URL.String()); key != "" {
			// get the whitelist for this endpoint and check any further restrictions
			if awl, ok := ah.whitelist.Get(key).(*whitelistEntry); ok {
				// if we have no IP/host restrictions chain the request
				if (len(awl.nets) == 0) && (len(awl.hosts) == 0) {
					h.ServeHTTP(w, r)
					return
				}

				// check further IP/host restrictions
				if remoteIP, err := parseForwarderIP(r, appEnv); err == nil {
					if awl.allows(remoteIP) {
						h.ServeHTTP(w, r)
						return
					}
				}
			}
		}
		// at this point the remote IP is not in the whitelist, or the api
		// endpoint doesn't have an entry
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	})
}
