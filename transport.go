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
	if t.tls {
		r.Header.Set("X-Forwarded-Proto", "https")
	}

	resp, err := t.tr.RoundTrip(r)
	if err == nil {
		if t.cors != nil && len(t.cors) > 0 {
			resp.Header.Set("Access-Control-Allow-Origin", strings.Join(t.cors, " "))
		}
		resp.Header.Set("Server", "Liberty")
		resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
		resp.Header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	return resp, err
}
