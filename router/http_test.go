package router

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.scot/liberty/middleware"
)

/*func TestExactMatch(t *testing.T) {
	router := &HTTPRouter{}
	mux := http.NewServeMux()
	sg := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux}, handler: mux}}}
	if err := router.put("http://www.example.com", sg); err != nil {
		t.Errorf("insertion error: foo")
	}
	if match := router.Getc("http://www.example.com"); match == nil {
		t.Errorf("bad search: foo")
	}
}
*/

func newServerGroup() http.Handler {
	mux := http.NewServeMux()
	return mux
	//return &balancer.ServerGroup{servers: []*server{{s: &http.Server{Handler: mux}, handler: mux}}}
}

func httpWriterRequest(urlPath string) (http.ResponseWriter, *http.Request) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", urlPath, nil)
	return w, req
}

func newRouter() *HTTPRouter {
	router := NewHTTPRouter()
	router.Use(
		[]middleware.Chainable{&middleware.HelloWorld{}},
	)

	return router
}

func TestRouteMatch(t *testing.T) {
	router := newRouter()
	mux := http.NewServeMux()

	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/example/path", mux); err != nil {
		t.Error(err)
	}
	match := router.match("/test/example/path", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)
}

func TestBenchMatchFail01(t *testing.T) {
	testPath := "/test/test/test/test/test"
	fiveColon := "/:a/:b/:c/:d/:e"

	router := newRouter()
	mux := http.NewServeMux()

	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle(fiveColon, mux); err != nil {
		t.Error(err)
	}
	match := router.match(testPath, ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)

}

func TestMatchLastVar(t *testing.T) {

	router := NewHTTPRouter()
	mux := http.NewServeMux()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	//w := httptest.NewRecorder()

	if err := router.Handle("/test/:var1", mux); err != nil {
		t.Error(err)
	}

	match := router.match("/test/foo", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)
}

func TestRouteMatchOneVar(t *testing.T) {

	router := NewHTTPRouter()
	mux := http.NewServeMux()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/:varone/bar", mux); err != nil {
		t.Error(err)
	}

	match := router.match("/test/foo/bar", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)
}

func TestRouteMatchTwoVar(t *testing.T) {

	router := NewHTTPRouter()
	mux := http.NewServeMux()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/example/:var1/path/:var2", mux); err != nil {
		t.Error(err)
	}

	match := router.match("/test/example/foobar/path/barbaz", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)
}

func TestRouteMatchLongest(t *testing.T) {

	router := &HTTPRouter{}
	mux := http.NewServeMux()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	router.Handle("http://www.example.com/*", mux)
	match := router.match("http://www.example.com/foo/", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}
	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)
}

func TestBenchFail(t *testing.T) {
	router := &HTTPRouter{}
	mux := http.NewServeMux()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	testRoute := "/users/:user/following"
	router.Handle(testRoute, mux)
	match := router.match("/users/foobar/following", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}
	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}

	ctxPool.Put(ctx)
}

/*
func TestLongesPrefixtMatch(t *testing.T) {
	router := &HTTPRouter{}
	mux1 := http.NewServeMux()
	mux2 := http.NewServeMux()
	h1 := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux1}, handler: mux1}}}
	h2 := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux2}, handler: mux2}}}
	router.put("http://www.example.com/", h1)
	router.put("http://www.example.com/foo/", h2)
	match := router.Getc("http://www.example.com/foo/bar")
	if match == nil {
		t.Errorf("bad search: no match")
	}
	if fmt.Sprintf("%p", mux2) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: h2: %#v,  match: %#v", mux2, match)
	}
}
*/

/*func BenchmarkTreePut(b *testing.B) {
	router := &HTTPRouter{}
	rand.Seed(42)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		key := []rune{}
		mux := http.NewServeMux()
		for n := 0; n < rand.Intn(1000); n++ {
			key = append(key, rune(rand.Intn(94)+32))
		}

		router.put(string(key), mux)
	}
}

func BenchmarkMapPut(b *testing.B) {
	hash := make(map[string]http.Handler)
	rand.Seed(42)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		key := []rune{}
		mux := http.NewServeMux()
		for n := 0; n < rand.Intn(1000); n++ {
			key = append(key, rune(rand.Intn(94)+32))
		}

		hash[string(key)] = mux
	}
}*/

func valuesForBenchmark(numValues int, cb func(string)) {
	rand.Seed(42)
	for i := 0; i < numValues; i++ {
		key := []rune{}
		if i == int(math.Floor(float64(numValues/2.0))) {
			key = []rune("www.match.com/api/path")
		} else {
			for j := 0; j < rand.Intn(1000)+1; j++ {
				key = append(key, rune(rand.Intn(94)+32))
			}
		}
		cb(string(key))
	}
}

func loadGithubApi(cb func(string) error) {
	for _, route := range githubAPI {
		cb(string(route.path))
	}
}

func BenchmarkTreeGet1000(b *testing.B) {
	router := newRouter()
	sg := newServerGroup()
	loadGithubApi(func(key string) error {
		return router.Handle(key, sg)
	})

	w, req := httpWriterRequest("/user/repos")

	b.ReportAllocs()
	b.ResetTimer()

	//ctx := ctxPool.Get().(*Context)
	//ctx.Reset()

	for n := 0; n < b.N; n++ {
		router.ServeHTTP(w, req)
		//	_ = router.match("/user/repos", ctx)
	}

	//ctxPool.Put(ctx)
}

func BenchmarkTreeGetVar1000(b *testing.B) {
	router := newRouter()
	sg := newServerGroup()

	loadGithubApi(func(key string) error {
		return router.Handle(key, sg)
	})

	w, req := httpWriterRequest("/user/repos")

	b.ReportAllocs()
	b.ResetTimer()

	//ctx := ctxPool.Get().(*Context)
	//ctx.Reset()

	for n := 0; n < b.N; n++ {
		router.ServeHTTP(w, req)
		//_ = router.match("/users/graham/gists", ctx)
	}

	//ctxPool.Put(ctx)
}
