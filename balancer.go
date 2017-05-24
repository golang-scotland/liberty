package liberty

import (
	"fmt"
	"net/http"

	"golang.scot/liberty/middleware"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/facebookgo/grace/gracenet"
)

type Config struct {
	Certs     []*Crt                     `yaml:"certs"`
	Proxies   []*ReverseProxy            `yaml:"proxies"`
	Whitelist []*middleware.ApiWhitelist `yaml:"whitelist"`
}

// Proxy is a balanced reverse HTTP proxy
type Proxy struct {
	config   *Config
	group    *ServerGroup
	secure   map[string]*VHost
	insecure map[string]*VHost
}

// NewProxy returns a Balancer configured for use
func NewProxy(config *Config) *Proxy {
	b := &Proxy{
		config:   config,
		secure:   map[string]*VHost{},
		insecure: map[string]*VHost{},
	}

	servers := make([]*http.Server, 0)

	for _, proxy := range b.config.Proxies {
		host, _ := proxy.hostAndPath()
		if _, ok := b.secure[host]; !ok {
			b.secure[host] = &VHost{
				host:    host,
				handler: NewRouter(),
			}
		}
		if _, ok := b.insecure[host]; !ok {
			b.insecure[host] = &VHost{
				host:    host,
				handler: http.HandlerFunc(middleware.RedirectPerm),
			}
		}

		err := proxy.Configure(b.config.Whitelist, b.secure[host].handler, b.serveInsecure)
		if err != nil {
			fmt.Printf("the proxy for '%s' was not configured - %s\n", proxy.HostPath, err)
			continue
		}

		servers = append(servers, proxy.Servers...)
	}

	b.group = NewServerGroup(b, servers)

	for _, s := range b.group.HTTPServers() {
		setTLSConfig(s, b.vhostDomains())
	}

	return b
}

func (b *Proxy) vhostDomains() []string {
	domains := make([]string, 0)
	for host, _ := range b.secure {
		domains = append(domains, host)
	}

	return domains
}

// Serve incoming requests between a set of configured reverse proxies, uses
// kernel SO_REUSEPORT which conveniently maps incoming connections to the least
// used socket
func (b *Proxy) Serve() error {
	gracenet.Flags = gracenet.FlagReusePort

	return gracehttp.Serve(b.group.HTTPServers()...)
}

func (b *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if vhost, ok := b.secure[r.Host]; ok {
		fmt.Println(vhost, r.URL.Path)
		vhost.handler.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

func (b *Proxy) serveInsecure(w http.ResponseWriter, r *http.Request) {
	if s, ok := b.insecure[r.Host]; ok {
		s.handler.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}
