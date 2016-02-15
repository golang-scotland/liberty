package liberty

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"sort"
	"sync"

	"github.com/kavu/go_reuseport"
)

type strategy int

const (
	BestEffort strategy = iota
	LeastUsed
)

type Balancer struct {
	*sync.Mutex
	Addr     string
	Errors   chan error
	listener net.Listener
	proxies  []*httputil.ReverseProxy
	servers  []*server
}

func NewBalancer(addr string, proxies []*httputil.ReverseProxy) *Balancer {
	b := &Balancer{
		Addr:    addr,
		Errors:  make(chan error),
		proxies: proxies,
		servers: make([]*server, len(proxies), len(proxies)),
	}
	for i, proxy := range b.proxies {
		b.servers[i] = &server{0, &http.Server{Handler: proxy}}
		b.servers[i].ConnState = b.servers[i].trackState
	}
	return b
}

// Balance incoming requests between a set of configured reverse proxies using
// the desired balancing strategy.
func (b *Balancer) Balance(strat strategy) (ready chan struct{}) {
	ready = make(chan struct{})
	switch strat {
	case BestEffort:
		go b.bestEffort(ready)
	}
	return ready
}

// the bestEffort balancer leaves all the heavy lifting to the kernel, using the
// 3.9+ SO_REUSEPORT socket configuration.
func (b *Balancer) bestEffort(ready chan struct{}) {
	for _, s := range b.servers {
		listener, err := reuseport.NewReusablePortListener("tcp4", b.Addr)
		if err != nil {
			panic(err)
		}
		go s.Serve(listener)
	}

	ready <- struct{}{}

	for {
		select {
		case err := <-b.Errors:
			fmt.Println(err)
		}
	}
}

func leastUsedHandler(b *Balancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.leastUsedHandler().ServeHTTP(w, r)
	})
}

func (b *Balancer) leastUsedHandler() http.Handler {
	b.Lock()
	sort.Sort(b)
	b.Unlock()
	return b.servers[0].Handler
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
