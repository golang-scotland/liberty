package balancer

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"golang.scot/liberty/middleware"
)

func newProxy(addr string) *httputil.ReverseProxy {
	remote, err := url.Parse(addr)
	if err != nil {
		panic(err)
	}
	return httputil.NewSingleHostReverseProxy(remote)
}

func TestReusePort(t *testing.T) {

	numServers := 3
	addrs := make([]*net.TCPAddr, numServers)
	for i := 1; i <= numServers; i++ {
		v := i
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, v)
		}))
		server.Start()
		fmt.Printf("S%d URL: %s\n", i, server.URL)
		defer server.Close()

		url, _ := url.Parse(server.URL)

		addr, _ := net.ResolveTCPAddr("tcp", url.Host)
		addrs[i-1] = addr
	}

	conf := &Config{
		Proxies: []*middleware.Proxy{
			{
				HostPath:    "127.0.0.1:8989/",
				RemoteHost:  "127.0.0.1:3456",
				HostIP:      "127.0.0.1",
				HostPort:    8989,
				RemoteAddrs: addrs,
			},
		},
	}

	balancer := NewBalancer(conf)

	var balanceErr error
	go func() {
		balanceErr = balancer.Balance(Default)
		if balanceErr != nil {
			fmt.Sprintf("balancer server error: %s", balanceErr)
			t.Fatalf("balancer server error: %s", balanceErr)
		}
	}()
	time.Sleep(1 * time.Second)

	if balanceErr != nil {
		t.Error(balanceErr)
		return
	}

	tr := &http.Transport{}
	tr.DisableKeepAlives = true
	c := &http.Client{Transport: tr}

	var one, two, three int

	for i := 0; i <= 1; i++ {

		go func() {
			resp, err := c.Get("http://127.0.0.1:8989/")
			if err != nil {
				t.Fatalf("client get error: %s", err)
			}

			str, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
				t.Fatalf(err.Error())
			}
			//t.Logf("%d: %s\n", i, str)

			switch string(str) {
			case "1":
				one++
			case "2":
				two++
			case "3":
				three++
			}
		}()

	}

	time.Sleep(5 * time.Second)
	if one == 0 || two == 0 || three == 0 {
		t.Fail()
	}
}
