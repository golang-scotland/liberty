package liberty

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
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

	p1 := newProxy(s1.URL)
	p2 := newProxy(s2.URL)
	p3 := newProxy(s3.URL)

	proxies := []*httputil.ReverseProxy{
		p1,
		p2,
		p3,
	}

	tr := &http.Transport{}
	tr.DisableKeepAlives = true
	c := &http.Client{Transport: tr}

	bAddr := "127.0.0.1:8989"
	balancer := NewBalancer(bAddr, proxies)

	ready := balancer.Balance(BestEffort)
	<-ready

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
