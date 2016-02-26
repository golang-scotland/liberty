package balancer

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gnanderson/trie"
	"github.com/prometheus/client_golang/prometheus"
)

// Proxy defines the configuration of a reverse proxy entry in the router.
type Proxy struct {
	HostPath      string `yaml:"hostPath"`
	RemoteHost    string `yaml:"remoteHost"`
	remoteHostURL *url.URL
	remoteHostIPs []net.IP
	HostAlias     []string `yaml:"hostAlias"`
	HostIP        string   `yaml:"hostIP"`
	HostPort      int      `yaml:"hostPort"`
	Tls           bool     `yaml:"tls"`
	TlsRedirect   bool     `yaml:"tlsRedirect"`
	Ws            bool     `yaml:"ws"`
	HandlerType   string   `yaml:"handlerType"`
	IPs           []string `yaml:"ips, flow"`
	Cors          []string `yaml:"cors, flow"`
	servers       []*http.Server
}

var muxers map[int]*http.ServeMux

func getMux(port int) *http.ServeMux {
	if muxers == nil {
		muxers = make(map[int]*http.ServeMux)
	}
	if m, ok := muxers[port]; ok {
		return m
	}
	muxers[port] = http.NewServeMux()
	return muxers[port]
}

// Configure a proxy for use with the paramaters from the parsed yaml config. If
// a remote host resolves to more than one IP address, we'll create a server and
// for each. This works because under the hood we're using SO_REUSEPORT.
func (p *Proxy) Configure() error {
	p.normalise()
	if err := p.parseRemoteHost(); err != nil {
		return err
	}
	if err := p.initServers(); err != nil {
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
	p.remoteHostURL = remote

	// to support load balancing we're going to be creating multiple servers
	// in this scenario we'll need a slice of ip strings and we'll also be
	// explicit about the remote port.
	chunks := strings.Split(remote.Host, ":")
	var hostName string
	if len(chunks) > 1 {
		hostName = chunks[0]
	} else {
		hostName = p.remoteHostURL.Host

		// add the port - we'll need it to assemble remote URL based on ip
		var port int
		if p.Tls {
			port = 443
		} else {
			port = 80
		}
		remote.Host = fmt.Sprintf("%s:%d", p.remoteHostURL.Host, port)
	}
	ips, err := net.LookupIP(hostName)
	if err != nil {
		return fmt.Errorf("error in IP lookup for remote host '%s' - %s", hostName, err)
	}
	p.remoteHostIPs = ips
	fmt.Printf("IP's for proxy backend: %#v\n", p.remoteHostIPs)
	return nil
}

func (p *Proxy) initServers() error {
	// add an additional redirect from port 80
	if p.TlsRedirect && p.HostPort == 443 {
		s := &http.Server{
			Addr: fmt.Sprintf("%s:80", p.HostIP),
		}
		mux := http.NewServeMux()
		mux.HandleFunc(p.HostPath, redir)
		s.Handler = mux
		p.servers = append(p.servers, s)
	}

	// now the server (or servers) for this proxy entry
	for _, ip := range p.remoteHostIPs {
		s := &http.Server{
			Addr: fmt.Sprintf("%s:%d", p.HostIP, p.HostPort),
		}
		//mux := getMux(p.HostPort)
		mux := http.NewServeMux()

		if p.Tls {
			setTLSConfig(s)
		}

		// if this is a websocket proxy, we need to hijack the connection. We'll
		// have to treat this a little differently.
		if p.Ws {
			mux.Handle(p.HostPath, websocketProxy(p.RemoteHost))
			s.Handler = mux
			p.servers = append(p.servers, s)
			continue
		}

		// now configure the reverse proxy
		_, port, err := net.SplitHostPort(p.remoteHostURL.Host)
		if err != nil {
			return fmt.Errorf("remote host '%s' has an invalid format - %s", p.remoteHostURL.Host, err)
		}
		remoteShard := strings.Replace(p.RemoteHost, p.remoteHostURL.Host, fmt.Sprintf("%s:%s", ip.String(), port), 1)
		fmt.Printf("remote shard url: %s\n", remoteShard)
		err = reverseProxy(p, mux, remoteShard)
		if err != nil {
			return err
		}

		s.Handler = mux
		p.servers = append(p.servers, s)
	}
	return nil
}

// build a chain of handlers, with the last one actually performing the reverse
// proxy to the remote resource.
func reverseProxy(p *Proxy, mux *http.ServeMux, remoteUrl string) error {

	// if this remote host is not a valid resource we can't continue
	remote, err := url.Parse(remoteUrl)
	if err != nil {
		return fmt.Errorf("cannot parse remote url '%s' - %s", remoteUrl, err)
	}

	fmt.Printf("reverse proxying to: %#v\n", remote)

	// the first handler should be a prometheus instrumented handler
	handlers := make([]Chainable, 0)
	handlers = append(handlers, &InstrumentedHandler{Name: p.HostPath})

	// next we check for restrictions based on location / IP
	if len(p.IPs) > 0 {
		nets := ips2nets(p.IPs)
		restricted := &IPRestrictedHandler{Allowed: nets}
		restricted.handlerType = p.HandlerType

		// if this is also an API handler, pass in the open paths
		if restricted.handlerType == apiHandler {
			restricted.openPaths = &trie.Trie{}
			for _, wl := range conf.Whitelist {
				restricted.openPaths.Put(wl.Path, true)
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

	// now we should decided what type of resource the request is for, there's
	// only really three basic types at the moment: web, api, metrics
	var final http.Handler
	switch p.HandlerType {
	default:
		final = GoGetHandler(reverseProxy)
	case apiHandler:
		handlers = append(handlers, NewApiHandler(p))
		final = reverseProxy
	case promHandler:
		final = prometheus.InstrumentHandler(hostname, prometheus.Handler())
	case redirectHandler:
		final = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("%s/%s", p.RemoteHost, r.URL.Path), 302)
		})
	}

	// link the handler chain
	chain := NewChain(handlers...).Link(final)
	if len(p.HostAlias) > 0 {
		chunks := strings.Split(p.HostPath, ".")
		for _, alias := range p.HostAlias {
			chunks[0] = alias
			mux.Handle(strings.Join(chunks, "."), chain)
		}
	} else {
		mux.Handle(p.HostPath, chain)
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
