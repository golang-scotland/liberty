package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gnanderson/trie"
)

// Proxy defines the configuration of a reverse proxy entry in the router.
type Proxy struct {
	HostPath      string `yaml:"hostPath"`
	RemoteHost    string `yaml:"remoteHost"`
	remoteHostURL *url.URL
	RemoteAddrs   []*net.TCPAddr
	HostAlias     []string `yaml:"hostAlias"`
	HostIP        string   `yaml:"hostIP"`
	HostPort      int      `yaml:"hostPort"`
	Tls           bool     `yaml:"tls"`
	TlsRedirect   bool     `yaml:"tlsRedirect"`
	Ws            bool     `yaml:"ws"`
	HandlerType   string   `yaml:"handlerType"`
	IPs           []string `yaml:"ips, flow"`
	Cors          []string `yaml:"cors, flow"`
	Servers       []*http.Server
}

// Configure a proxy for use with the paramaters from the parsed yaml config. If
// a remote host resolves to more than one IP address, we'll create a server and
// for each. This works because under the hood we're using SO_REUSEPORT.
func (p *Proxy) Configure(whitelist []*ApiWhitelist) error {
	p.normalise()
	if err := p.parseRemoteHost(); err != nil {
		return err
	}
	if err := p.initServers(whitelist); err != nil {
		return err
	}
	return nil
}

// set port and scheme defaults
func (p *Proxy) normalise() {
	switch {
	case !p.Tls && p.HostPort == 0:
		p.HostPort = 80
	case p.Tls && p.HostPort == 0:
		p.HostPort = 443
	}

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

func (p *Proxy) parseRemoteHost() error {

	// TODO skip this proxy if error here?
	remote, err := url.Parse(p.RemoteHost)
	if err != nil {
		return fmt.Errorf("remote host URL could not be parsed - %s", err)
	}

	// to support load balancing we're going to be creating multiple servers
	// in this scenario we'll need a slice of ip strings and we'll also be
	// explicit about the remote port.
	chunks := strings.Split(remote.Host, ":")
	var remoteHost string
	var remotePort string

	if len(chunks) > 1 {
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
	// hose name to hanve one or more IP records in DNS or /hosts
	ips, err := net.LookupIP(remoteHost)
	if err != nil {
		return fmt.Errorf("error in IP lookup for remote host '%s' - %s", remoteHost, err)
	}

	if p.RemoteAddrs == nil {
		addrs := make([]*net.TCPAddr, len(ips), len(ips))
		for i, ip := range ips {
			addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", ip, remotePort))
			if err != nil {
				return fmt.Errorf("cannot parse address, port: '%s', port: %s, err: ", ip, remotePort, err)
			}
			addrs[i] = addr
		}
		p.RemoteAddrs = addrs
	}
	fmt.Printf("Backend IP's for proxy: %#s\n", p.RemoteAddrs)

	return nil
}

func (p *Proxy) initServers(whitelist []*ApiWhitelist) error {
	// add an additional redirect from port 80
	if p.TlsRedirect && p.HostPort == 443 {
		s := &http.Server{
			Addr: fmt.Sprintf("%s:80", p.HostIP),
		}
		mux := http.NewServeMux()
		mux.HandleFunc(p.HostPath, redir)
		s.Handler = mux
		p.Servers = append(p.Servers, s)
	}

	// now the server (or servers) for this proxy entry
	for _, addr := range p.RemoteAddrs {
		s := &http.Server{
			Addr: fmt.Sprintf("%s:%d", p.HostIP, p.HostPort),
		}
		mux := http.NewServeMux()

		// if this is a websocket proxy, we need to hijack the connection. We'll
		// have to treat this a little differently.
		if p.Ws {
			mux.Handle(p.HostPath, websocketProxy(p.RemoteHost))
			s.Handler = mux
			p.Servers = append(p.Servers, s)
			continue
		}

		remoteShard := strings.Replace(p.RemoteHost, p.remoteHostURL.Host, addr.String(), 1)
		fmt.Printf("remote shard url: %s\n", remoteShard)
		err := reverseProxy(p, mux, remoteShard, whitelist)
		if err != nil {
			return err
		}

		s.Handler = mux
		p.Servers = append(p.Servers, s)
	}

	return nil
}

// build a chain of handlers, with the last one actually performing the reverse
// proxy to the remote resource.
func reverseProxy(p *Proxy, mux *http.ServeMux, remoteUrl string, whitelist []*ApiWhitelist) error {

	// if this remote host is not a valid resource we can't continue
	remote, err := url.Parse(remoteUrl)
	if err != nil {
		return fmt.Errorf("cannot parse remote url '%s' - %s", remoteUrl, err)
	}

	fmt.Printf("reverse proxying to: %#v\n", remote)

	// the first handler should be a prometheus instrumented handler
	handlers := make([]Chainable, 0)

	// @DEPRECATED https://github.com/prometheus/client_golang/issues/200
	//handlers = append(handlers, &InstrumentedHandler{Name: p.HostPath})

	// next we check for restrictions based on location / IP
	if len(p.IPs) > 0 {
		nets := ips2nets(p.IPs)
		restricted := &IPRestrictedHandler{Allowed: nets}
		restricted.handlerType = p.HandlerType

		// if this is also an API handler, pass in the open paths
		if restricted.handlerType == apiHandler {
			restricted.openPaths = &trie.Trie{}
			for _, wl := range whitelist {
				restricted.openPaths.Put(wl.Path, true)
			}
		}
		handlers = append(handlers, restricted)
	}
	// ?

	// use a standard library reverse proxy, but use our own transport so that
	// we can further update the response
	reverseProxy := httputil.NewSingleHostReverseProxy(remote)
	reverseProxy.Transport = &Transport{
		tr:   http.DefaultTransport,
		tls:  p.Tls,
		cors: p.Cors,
	}

	// now we should decided what type of resource the request is for, there's
	// only really three basic types at the moment: web, api, metrics
	var final http.Handler
	switch p.HandlerType {
	default:
		//final = GoGetHandler(reverseProxy)
		final = reverseProxy
	case apiHandler:
		handlers = append(handlers, NewApiHandler(whitelist))
		final = reverseProxy
	/*
		case promHandler:
			final = prometheus.InstrumentHandler(env.Hostname(), prometheus.Handler())
	*/
	case redirectHandler:
		final = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("%s/%s", p.RemoteHost, r.URL.Path), 302)
		})
	}

	// link the handler chain
	chain := NewChain(handlers...).Link(final)
	mux.Handle(p.HostPath, chain)
	if len(p.HostAlias) > 0 {
		chunks := strings.Split(p.HostPath, ".")
		for _, alias := range p.HostAlias {
			chunks[0] = alias
			mux.Handle(strings.Join(chunks, "."), chain)
		}
	}
	return nil
}

// convert a list of IP address strings in CIDR format to IPNets
func ips2nets(ips []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0)
	for _, ipRange := range ips {
		_, ipNet, err := net.ParseCIDR(ipRange)
		if err != nil {
			panic(err)
		}
		nets = append(nets, ipNet)
	}
	return nets
}
