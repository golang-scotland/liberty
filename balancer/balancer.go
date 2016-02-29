package balancer

import (
	"fmt"
	"net/http"

	"golang.scot/liberty/router"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/facebookgo/grace/gracenet"
)

type strategy int

const (
	Default strategy = iota
)

type Balancer struct {
	config *Config
	groups map[string]*router.ServerGroup
	router *router.HttpRouter
}

func NewBalancer() *Balancer {
	b := &Balancer{
		config: conf,
		groups: map[string]*router.ServerGroup{},
		router: &router.HttpRouter{},
	}
	for _, proxy := range b.config.Proxies {
		err := proxy.Configure()
		if err != nil {
			fmt.Printf("the proxy for '%s' was not configured - %s", proxy.HostPath, err)
			continue
		}
		b.groups[proxy.HostPath] = router.NewServerGroup(b.router, proxy.servers)
		err = b.router.Put(proxy.HostPath, b.groups[proxy.HostPath])
		if err != nil {
			fmt.Printf("unable to register the HostPath '%s' with this route", proxy.HostPath)
			continue
		}
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
	servers := make([]*http.Server, 0)
	for _, sg := range b.groups {
		servers = append(servers, sg.HttpServers()...)
	}
	return gracehttp.Serve(servers...)
}
