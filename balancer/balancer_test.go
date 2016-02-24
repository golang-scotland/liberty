package balancer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"
)

func newProxy(addr string) *httputil.ReverseProxy {
	remote, err := url.Parse(addr)
	if err != nil {
		panic(err)
	}
	return httputil.NewSingleHostReverseProxy(remote)
}

func TestReusePort(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "1")
	}))
	defer s1.Close()
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "2")
	}))
	defer s2.Close()
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "3")
	}))
	defer s3.Close()

	tr := &http.Transport{}
	tr.DisableKeepAlives = true
	c := &http.Client{Transport: tr}

	balancer := NewBalancer()

	balancer.config.Proxies = []*Proxy{
		{
			HostPath:   "/",
			RemoteHost: s1.URL,
			HostIP:     "127.0.0.1",
			HostPort:   8989,
		},
		{
			HostPath:   "/",
			RemoteHost: s2.URL,
			HostIP:     "127.0.0.1",
			HostPort:   8989,
		},
		{
			HostPath:   "/",
			RemoteHost: s3.URL,
			HostIP:     "127.0.0.1",
			HostPort:   8989,
		},
	}

	var balanceErr error
	go func() {
		balanceErr = balancer.Balance(BestEffort)
	}()
	time.Sleep(2 * time.Second)

	if balanceErr != nil {
		t.Error(balanceErr)
		return
	}

	var one, two, three int

	for i := 0; i <= 100; i++ {

		resp, err := c.Get("http://127.0.0.1:8989/")
		if err != nil {
			t.Fatalf("client get error: %s", err)
		}

		str, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
			t.Fatalf(err.Error())
		}
		t.Logf("%d: %s\n", i, str)

		switch string(str) {
		case "1":
			one++
		case "2":
			two++
		case "3":
			three++
		}

	}

	if one == 0 || two == 0 || three == 0 {
		t.Fail()
	}
}
