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
}

func NewBalancer() *Balancer {
	b := &Balancer{
		Errors:  make(chan error),
		servers: make([]*server, 0),
	}

	for _, proxy := range conf.Proxies {
		servers, err := proxy.configure()
		if err != nil {
			panic(err)
		}
		for i, s := range servers {
			b.servers = append(b.servers, &server{0, s})
			b.servers[i].s.ConnState = b.servers[i].trackState
		}
	}
	return b
}

// Balance incoming requests between a set of configured reverse proxies using
// the desired balancing strategy.
func (b *Balancer) Balance(strat strategy) {
	switch strat {
	case BestEffort:
		b.bestEffort()
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
func (b *Balancer) bestEffort() {
	gracenet.Flags = gracenet.FlagReusePort
	err := gracehttp.Serve(b.Servers()...)
	if err != nil {
		fmt.Println(err)
	}
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
