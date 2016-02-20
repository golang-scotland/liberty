package liberty

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/gnanderson/trie"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"golang.scot/pkg/env"
)

const (
	apiHandler      = "api"
	webHandler      = "web"
	promHandler     = "prometheus"
	redirectHandler = "redirect"
)

// Chainable describes a handler which wraps a handler. By design there is no
// guarantee that a chainable handler will call the next one in the chain. To
// be chainable the object must also be able to serve HTTP requests and thus
// it will also itself satisfy the standard library http.Handler interface
type Chainable interface {
	Chain(h http.Handler) http.Handler
}

// Chain is a series of chainable http handlers
type Chain struct {
	handlers []Chainable
}

// NewChain initiates the chain
func NewChain(handlers ...Chainable) Chain {
	ch := Chain{}
	ch.handlers = append(ch.handlers, handlers...)
	return ch
}

// Link the chain
func (ch Chain) Link(h http.Handler) http.Handler {
	var last http.Handler

	if h == nil {
		last = http.DefaultServeMux
	} else {
		last = h
	}

	for i := len(ch.handlers) - 1; i >= 0; i-- {
		last = ch.handlers[i].Chain(last)
	}

	return last
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello World!"))
}

type GzipHandler struct{}

func (hl *GzipHandler) Chain(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gziphandler.GzipHandler(h)
	})
}

// IPRestrictedHandler does what it says on the tin - names not down, not getting
// in!
type IPRestrictedHandler struct {
	Allowed     []*net.IPNet
	handlerType string
	openPaths   *trie.Trie
}

func (rh *IPRestrictedHandler) Chain(h http.Handler) http.Handler {
	appEnv := env.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := parseForwarderIP(r, appEnv)
		if err != nil {
			glog.Errorln(err)
			http.Error(w, err.Error(), 500)
			return
		}

		// don't block any dev environment
		/*
			if appEnv == env.Dev {
				h.ServeHTTP(w, r)
				return
			}
		*/

		// if this is an API handler we need to restrict access by default
		// unless the request path has a prefix in the trie of open paths. The
		// order of access should be...
		//
		// 1. Open paths are allowed
		// 2. If the path is not open but the IP is allowed, proceed
		if rh.handlerType == apiHandler && rh.openPaths.LongestPrefix(r.URL.String()) != "" {
			if glog.V(3) {
				glog.Infof("Open API: %s - %s%s", ip, r.Host, r.URL.String())
			}
			h.ServeHTTP(w, r)
			return
		}

		for _, ipNet := range rh.Allowed {
			if ipNet.Contains(ip) {
				if glog.V(3) {
					glog.Infof("API IP Access: %s - %s%s", ip, r.Host, r.URL.String())
				}
				h.ServeHTTP(w, r)
				return
			}
		}

		// At this point it could be a paypal IPN notification, so we'll do one
		// final check before returning a 403. Until we have better test coverage
		// please leave this paypal specific chunk of code in place. This final
		// check does a reverse lookup on the incoming IP address and if it matches
		// paypal.com then we serve the request.
		if !(appEnv == env.Dev) {
			if ValidIPNSource(ip) {
				h.ServeHTTP(w, r)
				return
			}
		}

		glog.Infof("Blocked: %s - %s%s", ip, r.Host, r.URL.String())
		glog.Infof("Referrer: %s", r.Header.Get("Referer"))

		http.Error(w, fmt.Sprintf("IP %s is not allowed...", ip), 403)
	})
}

// try to retrive the forwarding IP
func parseForwarderIP(r *http.Request, appEnv env.Env) (ip net.IP, err error) {
	var remote string
	// Outside of production env we always use the remote address
	if appEnv != env.Prod {
		remote, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return nil, err
		}
	} else {
		remote = r.Header.Get("X-Forwarded-For")
		if remote == "" {
			remote, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return nil, err
			}
		}
		// some forwarded for headers contain a sequence of IP's, in this case
		// we are interested in the first one.
		if strings.Contains(remote, ",") {
			remote = strings.Split(remote, ",")[0]
		}
	}

	return net.ParseIP(remote), nil
}

// ValidIPNSource checks whether the request originates at paypal
func ValidIPNSource(ip net.IP) bool {
	if names, err := net.LookupAddr(ip.String()); err == nil {
		for _, name := range names {
			if glog.V(1) {
				glog.Infof("IP Reverse Name: %s", name)
			}
			if strings.HasSuffix(name, "paypal.com.") {
				return true
			}
		}
	}
	return false
}

// InstrumentedHandler often starts the handler chain by initialising some
// prometheus based monitoring and metrics.
type InstrumentedHandler struct {
	Name string
}

func (ih *InstrumentedHandler) Chain(h http.Handler) http.Handler {
	return http.HandlerFunc(prometheus.InstrumentHandler(ih.Name, h))
}

// ApiHandler further restricts access to the API. Only certain endpoints are
// open by default, and even open paths can have further IP or hostname based
// restrictions. This is the second layer of the onion, the first potentially
// being an IPRestrictedHandler
type ApiHandler struct {
	Whitelist *trie.Trie
}

// an apiWhitelist is a slice of allowed ip nets and/or a slice of remote hostnames
type apiWhitelist struct {
	ips   []*net.IPNet
	hosts []string
}

func (a *apiWhitelist) hasIP(ip net.IP) bool {
	for _, ipNet := range a.ips {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
func (a *apiWhitelist) hasHost(ip net.IP) bool {
	if len(a.hosts) == 0 {
		return false
	}
	names, err := net.LookupAddr(ip.String())
	if err != nil {
		glog.Warningf("Reverse IP lookup error: %s", err)
		return false
	}

	for _, host := range a.hosts {
		for _, name := range names {
			if glog.V(1) {
				glog.Infof("IP reverse name check - name: %s, host: %s", name, host)
			}
			if strings.HasSuffix(name, host) {
				return true
			}
		}
	}
	return false
}

func (a *apiWhitelist) allows(ip net.IP) bool {
	return a.hasIP(ip) || a.hasHost(ip)
}

func NewApiHandler(p *Proxy) *ApiHandler {
	openAPI = &trie.Trie{}
	for _, wl := range conf.Whitelist {
		nets := ips2nets(wl.IPs)
		awl := &apiWhitelist{nets, wl.Hostnames}
		openAPI.Put(wl.Path, awl)
	}
	return &ApiHandler{Whitelist: openAPI}
}

func (ah *ApiHandler) Chain(h http.Handler) http.Handler {
	appEnv := env.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if url path is a sub path of a whitelisted prefex e.g. the path is
		// /api/foo/bar and the whitelist contains /api/foo then this will be
		// allowed. Similarly we will proceed if the path and registered path
		// prefix match exactly.
		if key := ah.Whitelist.LongestPrefix(r.URL.String()); key != "" {
			// get the whitelist for this endpoint and check any further restrictions
			if awl, ok := ah.Whitelist.Get(key).(*apiWhitelist); ok {
				// if we have no IP/host restrictions chain the request
				if (len(awl.ips) == 0) && (len(awl.hosts) == 0) {
					h.ServeHTTP(w, r)
					return
				}

				// check further IP/host restrictions
				if remoteIP, err := parseForwarderIP(r, appEnv); err == nil {
					if awl.allows(remoteIP) {
						h.ServeHTTP(w, r)
						return
					}
					glog.Infof("Blocked: %s - %s%s", remoteIP, r.Host, r.URL.String())
					glog.Infof("Referrer: %s", r.Header.Get("Referer"))
				}
			}
		}
		// at this point the remote IP is not in the whitelist, or the api
		// endpoint doesn't have an entry
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	})
}
func redir(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("https://%s%s", r.Host, r.RequestURI)
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}

// hijack the connection and dial the backend to do the HTTP upgrade dance
func websocketProxy(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, "Error contacting backend server.", 500)
			log.Printf("Error dialing websocket backend %s: %v", target, err)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Not a hijacker?", 500)
			return
		}
		nc, _, err := hj.Hijack()
		if err != nil {
			log.Printf("Hijack error: %v", err)
			return
		}
		defer nc.Close()
		defer d.Close()

		err = r.Write(d)
		if err != nil {
			log.Printf("Error copying request to target: %v", err)
			return
		}

		errc := make(chan error, 2)
		cp := func(dst io.Writer, src io.Reader) {
			_, err := io.Copy(dst, src)
			errc <- err
		}
		go cp(d, nc)
		go cp(nc, d)
		<-errc
	})
}
