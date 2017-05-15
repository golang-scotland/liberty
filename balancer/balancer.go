package balancer

import (
	"fmt"
	"net/http"

	"golang.scot/liberty/middleware"
	"golang.scot/liberty/router"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/facebookgo/grace/gracenet"
)

type strategy int

const (
	// Default is the base and fallback balancing strategy
	Default strategy = iota
)

// Balancer is a balanced reverse HTTP proxy
type Balancer struct {
	config *Config
	group  *ServerGroup
	router *router.Router
}

type Config struct {
	Certs     []*Crt                     `yaml:"certs"`
	Proxies   []*middleware.Proxy        `yaml:"proxies"`
	Whitelist []*middleware.ApiWhitelist `yaml:"whitelist"`
}

// NewBalancer returns a Balancer configured for use
func NewBalancer(config *Config) *Balancer {
	b := &Balancer{
		config: config,
		group:  *ServerGroup,
		router: router.NewRouter(),
	}

	servers := make([]*http.Server, 0)

	for _, proxy := range b.config.Proxies {
		err := proxy.Configure(b.config.Whitelist, b.router)
		if err != nil {
			fmt.Printf("the proxy for '%s' was not configured - %s", proxy.HostPath, err)
			continue
		}

		// set TLS options on the proxy servers
		if proxy.Tls {
			for i := range proxy.Servers {
				setTLSConfig(proxy.Servers[i], config.Certs)
			}
		}

		servers = append(servers, proxy.Servers...)
	}

	return b
}

// Balance incoming requests between a set of configured reverse proxies using
// the desired balancing strategy.
func (b *Balancer) Balance(strat strategy) error {
	switch strat {
	default:
		return b.bestEffort()
	}
}

func (b *Balancer) bestEffort() error {
	// this toggles SO_REUSEPORT
	gracenet.Flags = gracenet.FlagReusePort
	var servers []*http.Server

	for _, sg := range b.groups {
		servers = append(servers, sg.HTTPServers()...)
	}

	return gracehttp.Serve(servers...)
}
