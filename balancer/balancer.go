package balancer

import (
	"fmt"
	"net/http/httputil"

	"golang.scot/liberty/router"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/facebookgo/grace/gracenet"
	"github.com/golang/glog"
)

type strategy int

const (
	Default strategy = iota
)

type Balancer struct {
	proxies []*httputil.ReverseProxy
	sg      *router.ServerGroup
	config  *Config
}

func NewBalancer() *Balancer {
	b := &Balancer{
		config: conf,
	}
	return b
}

// Balance incoming requests between a set of configured reverse proxies using
// the desired balancing strategy.
func (b *Balancer) Balance(strat strategy) error {
	for _, proxy := range b.config.Proxies {
		err := proxy.Configure()
		if err != nil {
			fmt.Printf("the proxy for '%s' was not configured - %s", proxy.HostPath, err)
			continue
		}
		b.sg = router.NewServerGroup(proxy.servers)
	}

	switch strat {
	default:
		return b.bestEffort()
	}
}

// the bestEffort balancer leaves all the heavy lifting to the kernel, using the
// 3.9+ SO_REUSEPORT socket configuration.
func (b *Balancer) bestEffort() error {
	gracenet.Flags = gracenet.FlagReusePort
	servers := b.sg.HttpServers()
	for _, s := range servers {
		glog.Infof("%#v", s.Handler)
	}

	return gracehttp.Serve(servers...)
}
