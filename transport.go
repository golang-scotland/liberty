package liberty

import (
	"net/http"
	"strings"
)

// Transport wraps a standard library http roundtripper
type Transport struct {
	tr   http.RoundTripper
	tls  bool
	cors []string
}

// RoundTrip sets some standard headers
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	// we're not really serving anything over port 80, so when we proxy always
	// send https as the scheme in the forwarded headers
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-For", r.RemoteAddr)

	resp, err := t.tr.RoundTrip(r)
	if err == nil {
		if t.cors != nil && len(t.cors) > 0 {
			resp.Header.Set("Access-Control-Allow-Origin", strings.Join(t.cors, " "))
		}
		resp.Header.Set("Server", "Liberty")
		resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
		// DANGER WILL ROBINSON
		//resp.Header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	return resp, err
}
