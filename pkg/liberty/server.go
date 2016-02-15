package liberty

import (
	"net"
	"net/http"
	"sync/atomic"
)

type server struct {
	open uint32
	*http.Server
}

func (s *server) openConns() uint32 {
	return atomic.LoadUint32(&s.open)
}

func (s *server) trackState(c net.Conn, cs http.ConnState) {
	switch cs {
	// Do we care more about StateNew than StateActive?
	case http.StateNew:
		atomic.AddUint32(&s.open, 1)
		// new/active connections will eventually transition to hijacked or closed and
	// with both being terminal states we will decrement the count on a callback
	// from either of these states. If your application makes use of websockets,
	// you should probably just use the best effor balancer...
	case http.StateClosed, http.StateHijacked:
		atomic.AddUint32(&s.open, ^uint32(0))
	}
}
