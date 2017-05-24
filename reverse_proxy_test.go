package liberty

import (
	"strings"
	"testing"
)

func TestNormaliseProxy(t *testing.T) {
	proxy := &ReverseProxy{
		Tls: false,
	}
	proxy.normalise()
	if proxy.HostPort != 80 {
		t.Errorf("proxy not normalised - host port is '%d', expected '80'. %#v", proxy.HostPort, proxy)
	}
	if proxy.HostIP != "0.0.0.0" {
		t.Errorf("proxy not normalised - unexpected host IP '%s'. %#v", proxy.HostIP, proxy)
	}

	proxy = &ReverseProxy{
		Tls: true,
	}
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
