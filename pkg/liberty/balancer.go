package liberty

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sort"
	"sync"

	"github.com/facebookgo/grace/gracehttp"
	"github.com/facebookgo/grace/gracenet"
	"github.com/golang/glog"
)

type strategy int

const (
	BestEffort strategy = iota
	LeastUsed
)

type Balancer struct {
	*sync.Mutex
	Errors   chan error
	listener net.Listener
	proxies  []*httputil.ReverseProxy
	servers  []*server
	config   *Config
}

func NewBalancer() *Balancer {
	b := &Balancer{
		Errors:  make(chan error),
		servers: make([]*server, 0),
		config:  conf,
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
		for i, s := range proxy.servers {
			b.servers = append(b.servers, &server{0, s})
			b.servers[i].s.ConnState = b.servers[i].trackState
		}
	}

	switch strat {
	default:
		return b.bestEffort()

	}
}

func (b *Balancer) Servers() []*http.Server {
	servers := make([]*http.Server, len(b.servers), len(b.servers))
	for i, s := range b.servers {
		servers[i] = s.s
	}
	return servers
}

// the bestEffort balancer leaves all the heavy lifting to the kernel, using the
// 3.9+ SO_REUSEPORT socket configuration.
func (b *Balancer) bestEffort() error {
	gracenet.Flags = gracenet.FlagReusePort
	servers := b.Servers()
	for _, s := range servers {
		glog.Infof("%#v", s.Handler)
	}

	return gracehttp.Serve(b.Servers()...)
}

func leastUsedHandler(b *Balancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.leastUsedHandler().ServeHTTP(w, r)
	})
}

func (b *Balancer) leastUsedHandler() http.Handler {
	var h http.Handler
	b.Lock()
	sort.Sort(b)
	h = b.servers[0].s.Handler
	b.Unlock()
	return h
}

func (b *Balancer) Len() int {
	return len(b.servers)
}

func (b *Balancer) Less(i, j int) bool {
	return b.servers[i].openConns() < b.servers[j].openConns()
}

func (b *Balancer) Swap(i, j int) {
	b.servers[i], b.servers[j] = b.servers[j], b.servers[i]
}
