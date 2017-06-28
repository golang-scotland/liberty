package liberty

import (
	"strings"
	"testing"
)

func TestNormaliseProxy(t *testing.T) {
	proxy := &ReverseProxy{}
	proxy.normalise()
	if proxy.HostPort != 443 {
		t.Errorf("proxy not normalised - host port is '%d', expected '443'. %#v", proxy.HostPort, proxy)
	}

	proxy = &ReverseProxy{
		Tls:        true,
		RemoteHost: "example.com",
	}
	proxy.normalise()
	if !strings.HasPrefix(proxy.RemoteHost, "https://") {
		t.Errorf("proxy not normalised - unepxected remote scheme %s%, %#v", proxy.RemoteHost, proxy)
	}

	proxy = &ReverseProxy{
		Tls:        false,
		RemoteHost: "example.com",
	}
	proxy.normalise()
	if !strings.HasPrefix(proxy.RemoteHost, "http://") {
		t.Errorf("proxy not normalised - unepxected remote scheme %s%, %#v", proxy.RemoteHost, proxy)
	}
}
