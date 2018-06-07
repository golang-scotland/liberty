package liberty

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"golang.scot/liberty/middleware"

	"github.com/gnanderson/trie"
)

// ReverseProxy defines the configuration of a reverse proxy entry in the router.
type ReverseProxy struct {
	HostPath      string `yaml:"hostPath"`
	RemoteHost    string `yaml:"remoteHost"`
	remoteHostURL *url.URL
	remoteAddrs   []*net.TCPAddr
	HostAlias     []string `yaml:"hostAlias"`
	HostIP        string   `yaml:"hostIP"`
	HostPort      int      `yaml:"hostPort"`
	Tls           bool     `yaml:"tls"`
	Ws            bool     `yaml:"ws"`
	HandlerType   string   `yaml:"handlerType"`
	GoGetOrg      string   `yaml:"goGetOrg"`
	IPs           []string `yaml:"ips, flow"`
	Cors          []string `yaml:"cors, flow"`
	Servers       []*http.Server
}

func (p *ReverseProxy) hostAndPath() (host string, path string) {
	fmt.Println("getting host and path for proxy: ", p.HostPath)
	chunks := strings.SplitN(p.HostPath, "/", 2)
	// if no path specified, use wildcard to serve all paths
	if len(chunks) < 2 {
		chunks = append(chunks, "*")
	}

	return chunks[0], "/" + chunks[1]
}

// Configure a proxy for use with the paramaters from the parsed yaml config. If
// a remote host resolves to more than one IP address, we'll create a server and
// for each. This works because under the hood we're using SO_REUSEPORT.
func (p *ReverseProxy) Configure(whitelist []*middleware.ApiWhitelist, router http.Handler) error {
	p.normalise()
	if err := p.parseRemoteHost(); err != nil {
		return err
	}

	return p.initServers(whitelist, router)
}

// set port and scheme defaults
func (p *ReverseProxy) normalise() {
	p.HostPort = 443

	if !strings.HasPrefix(p.RemoteHost, "http") {
		var scheme string
		if p.Tls {
			scheme = "https://"
		} else {
			scheme = "http://"
		}
		p.RemoteHost = fmt.Sprintf("%s%s", scheme, p.RemoteHost)
	}

	if p.HostIP == "" {
		p.HostIP = "0.0.0.0"
	}
}

func (p *ReverseProxy) parseRemoteHost() error {

	// TODO skip this proxy if error here?
	remote, err := url.Parse(p.RemoteHost)
	if err != nil {
		return fmt.Errorf("remote host URL could not be parsed - %s", err)
	}

	// to support load balancing we're going to be creating multiple servers
	// in this scenario we'll need a slice of ip strings and we'll also be
	// explicit about the remote port.
	var remoteHost string
	var remotePort string

	if strings.Contains(remote.Host, ":") {
		remoteHost, remotePort, err = net.SplitHostPort(remote.Host)
		if err != nil {
			return fmt.Errorf("remote host:port could not be parsed - %s", err)
		}
	} else {
		remoteHost = remote.Host

		// add the port - we'll need it to assemble remote URL based on ip
		if p.Tls {
			remotePort = "443"
		} else {
			remotePort = "80"
		}
		remote.Host = fmt.Sprintf("%s:%s", remoteHost, remotePort)
	}
	p.remoteHostURL = remote

	// now lookup the IP addresses for this, we would typically expect the remote
	// hose name to have one or more IP records in DNS or /etc/hosts
	ips, err := net.LookupIP(remoteHost)
	if err != nil {
		return fmt.Errorf("error in IP lookup for remote host '%s' - %s", remoteHost, err)
	}

	if p.remoteAddrs == nil {
		addrs := make([]*net.TCPAddr, len(ips), len(ips))
		for i, ip := range ips {
			addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", ip, remotePort))
			if err != nil {
				return fmt.Errorf("cannot parse address, port: '%s', port: %s, err: %s", ip, remotePort, err)
			}
			addrs[i] = addr
		}
		p.remoteAddrs = addrs
	}

	return nil
}

func (p *ReverseProxy) initServers(whitelist []*middleware.ApiWhitelist, router http.Handler) error {
	s := &http.Server{
		Addr: fmt.Sprintf("%s:80", p.HostIP),
	}
	p.Servers = append(p.Servers, s)

	for _, addr := range p.remoteAddrs {
		s := &http.Server{
			Addr: fmt.Sprintf("%s:%d", p.HostIP, p.HostPort),
		}

		remoteShard := strings.Replace(p.RemoteHost, p.remoteHostURL.Host, addr.String(), 1)
		err := reverseProxy(p, router, remoteShard, whitelist)
		if err != nil {
			return err
		}

		p.Servers = append(p.Servers, s)
	}

	return nil
}

// build a chain of handlers, with the last one actually performing the reverse
// proxy to the remote resource.
func reverseProxy(p *ReverseProxy, handler http.Handler, remoteUrl string, whitelist []*middleware.ApiWhitelist) (err error) {

	// if this remote host is not a valid resource we can't continue
	remote, err := url.Parse(remoteUrl)
	if err != nil {
		return fmt.Errorf("cannot parse remote url '%s' - %s", remoteUrl, err)
	}

	handlers := make([]middleware.Chainable, 0)

	// next we check for restrictions based on location / IP
	if len(p.IPs) > 0 {
		nets := middleware.IPs2nets(p.IPs)
		restricted := &middleware.IPRestrictedHandler{Allowed: nets}
		restricted.HandlerType = p.HandlerType

		// if this is also an API handler, pass in the open paths
		if restricted.HandlerType == middleware.ApiType {
			restricted.OpenPaths = &trie.Trie{}
			for _, wl := range whitelist {
				restricted.OpenPaths.Put(wl.Path, true)
			}
		}
		handlers = append(handlers, restricted)
	}

	// use a standard library reverse proxy, but use our own transport so that
	// we can further update the response
	reverseProxy := httputil.NewSingleHostReverseProxy(remote)
	reverseProxy.Transport = &Transport{
		tr:   http.DefaultTransport,
		tls:  p.Tls,
		cors: p.Cors,
	}

	// wrap the reverse proxy in a hijacker that will handle any upgrades to
	// websocket
	reverse := middleware.WebsocketProxy(p.RemoteHost, reverseProxy)

	// now we should decided what type of resource the request is for, there's
	// only really three basic types at the moment: web, api, metrics
	var final http.Handler
	switch p.HandlerType {
	default:
		final = reverse
	case middleware.BasicAuthType:
		final = middleware.BasicAuthHandler(reverse)
	case middleware.ApiType:
		handlers = append(handlers, middleware.NewApiHandler(whitelist))
		final = middleware.BasicAuthHandler(reverse)
	case middleware.GoGetType:
		gg := &middleware.GoGet{Org: p.GoGetOrg}
		final = gg.Handler(reverse)
	case middleware.RedirectType:
		final = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("%s/%s", p.RemoteHost, r.URL.Path), 302)
		})
	}

	// link the handler chain
	chain := middleware.NewChain(handlers...).Link(final)
	_, path := p.hostAndPath()

	router := handler.(*Router)
	router.All(path, chain)
	router.NotFound = chain

	return err
}
