package middleware

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/gnanderson/trie"
	"github.com/koding/websocketproxy"
	"github.com/prometheus/client_golang/prometheus"
	"golang.scot/liberty/env"
)

const (
	ApiType       = "api"
	WebType       = "web"
	PromType      = "prometheus"
	RedirectType  = "redirect"
	GoGetType     = "goget"
	BasicAuthType = "basic_auth"
)

type GzipHandler struct{}

func (hl *GzipHandler) Chain(h http.Handler) http.Handler {
	return gziphandler.GzipHandler(h)
}

// IPRestrictedHandler does what it says on the tin - names not down, not getting
// in!
type IPRestrictedHandler struct {
	Allowed     []*net.IPNet
	HandlerType string
	OpenPaths   *trie.Trie
}

func (rh *IPRestrictedHandler) Chain(h http.Handler) http.Handler {
	appEnv := env.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := parseForwarderIP(r, appEnv)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// If this is an API handler we need to restrict access by default
		// unless the request path has a prefix in the trie of open paths. The
		// order of access should be...
		//
		// 1. Open paths are allowed
		// 2. If the path is not open but the IP is allowed, proceed
		if rh.HandlerType == ApiType && rh.OpenPaths.LongestPrefix(r.URL.String()) != "" {
			h.ServeHTTP(w, r)
			return
		}

		for _, ipNet := range rh.Allowed {
			if ipNet.Contains(ip) {
				h.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, fmt.Sprintf("☹ - IP %s is not allowed...", ip), 403)
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

// validDomainSource checks whether the request originates at paypal
func validDomainSource(domain string, ip net.IP) bool {
	if names, err := net.LookupAddr(ip.String()); err == nil {
		for _, name := range names {
			if strings.HasSuffix(name, domain+".") {
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

func RedirectTemp(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("https://%s%s", r.Host, r.RequestURI)
	http.Redirect(w, r, url, 302)
}

func RedirectPerm(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("https://%s%s", r.Host, r.RequestURI)
	http.Redirect(w, r, url, 301)
}

func WebsocketProxy(target string, proxy http.Handler) http.Handler {
	remote, err := url.Parse(target)
	if err != nil {
		log.Fatal(err)
	}
	remote.Scheme = "wss"

	websocketproxy.DefaultDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	websocketProxy := websocketproxy.NewProxy(remote)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if !(strings.Contains(r.Header.Get("Connection"), "Upgrade") &&
			r.Header.Get("Upgrade") == "websocket") {
			proxy.ServeHTTP(w, r)

			return
		}
		fmt.Println("Websocket!")

		websocketProxy.ServeHTTP(w, r)
	})
}
