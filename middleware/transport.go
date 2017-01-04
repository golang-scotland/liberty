package middleware

import (
	"net/http"
	"strings"
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
