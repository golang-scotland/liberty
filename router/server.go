package router

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type server struct {
	open    uint32
	s       *http.Server
	handler http.Handler
}

func (s *server) openConns() uint32 {
	return atomic.LoadUint32(&s.open)
}

func (s *server) trackState(c net.Conn, cs http.ConnState) {
	switch cs {
	case http.StateNew:
		// Do we care more about StateNew than StateActive?
		atomic.AddUint32(&s.open, 1)
	case http.StateClosed, http.StateHijacked:
		// new/active connections will eventually transition to hijacked or closed and
		// with both being terminal states we will decrement the count on a callback
		// from either of these states.
		//atomic.AddUint32(&s.open, ^uint32(0))
	}
}

// ServerGroup defines a grouping of servers which can be have requests routed
// to via a liberty router
type ServerGroup struct {
	w       *sync.Mutex
	servers []*server
}

// NewServerGroup creates a server group from a router and a slice of standard
// library http servers
func NewServerGroup(router *HTTPRouter, servers []*http.Server) *ServerGroup {
	sg := &ServerGroup{
		w:       &sync.Mutex{},
		servers: []*server{},
	}
	for _, s := range servers {
		if strings.HasSuffix(s.Addr, "80") {
			continue
		}
		srv := &server{
			s:       s,
			handler: s.Handler,
		}
		s.ConnState = srv.trackState
		s.Handler = router
		sg.servers = append(sg.servers, srv)
	}
	//fmt.Sprintf("%#v\n", sg)
	return sg
}

// HTTPServers returns the standard library http servers associated with this
// ServerGroup
func (sg *ServerGroup) HTTPServers() []*http.Server {
	servers := make([]*http.Server, len(sg.servers), len(sg.servers))
	for i, s := range sg.servers {
		servers[i] = s.s
	}
	return servers
}

func (sg *ServerGroup) leastUsed() http.Handler {
	var h http.Handler
	if sg.w == nil {
		sg.w = &sync.Mutex{}
	}
	sg.w.Lock()
	sort.Sort(sg)
	h = sg.servers[0].handler
	sg.w.Unlock()
	return h
}

func (sg *ServerGroup) Len() int {
	return len(sg.servers)
}

func (sg *ServerGroup) Less(i, j int) bool {
	return sg.servers[i].openConns() < sg.servers[j].openConns()
}

func (sg *ServerGroup) Swap(i, j int) {
	sg.servers[i], sg.servers[j] = sg.servers[j], sg.servers[i]
}
