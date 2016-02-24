package liberty

import (
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

// Transport wraps a stadnard library http roundtripper
type Transport struct {
	tr   http.RoundTripper
	tls  bool
	cors []string
}

// RoundTrip sets some standard headers
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.tls {
		r.Header.Set("X-Forwarded-Proto", "https")
	}

	resp, err := t.tr.RoundTrip(r)
	if err == nil {
		if t.cors != nil && len(t.cors) > 0 {
			resp.Header.Set("Access-Control-Allow-Origin", strings.Join(t.cors, " "))
		}
		resp.Header.Set("Vary", "Accept-Encoding")
		resp.Header.Set("Server", "Liberty")
		resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
		resp.Header.Set("X-Powered-By", runtimeVer)
		resp.Header.Set("X-Id", hostname)
		/*setEtag(resp)
		if eTagMatch(r, resp) {
			return resp, nil
		}
		compress(r, resp)
		maxAge(resp)
		*/
	}
	return resp, err
}

type server struct {
	open uint32
	s    *http.Server
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
		// from either of these states. If your application makes use of websockets,
		// you should probably just use the best effor balancer...
		atomic.AddUint32(&s.open, ^uint32(0))
	}
}
