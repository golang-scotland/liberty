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

func newRouter() *Router {
	router := NewHTTPRouter()
	router.Use(
		[]middleware.Chainable{&middleware.HelloWorld{}},
	)

	return router
}

var testRoutes = []struct {
	m           method
	pattern     string
	path        string
	testMatches map[string]string
}{
	{GET, "/", "/", nil},
	{GET, "/test/example/path", "/test/example/path", nil},
	{GET, "/test/example/*", "/test/example/wildcard/test", nil},
	{GET, "/test/:var1", "/test/foo", map[string]string{
		"var1": "foo",
	}},
	{GET, "/:a/:b/:c/:d/:e", "/testa/test/testc/test/teste", map[string]string{
		"a": "testa",
		"c": "testc",
		"e": "teste",
	}},
	{GET, "/foo/:bar/baz", "/foo/:bar/baz", map[string]string{
		"bar": ":bar",
	}},
	{GET, "/repos/:owner/:repo/stargazers", "/repos/bob/liberty/stargazers", map[string]string{
		"owner": "bob",
		"repo":  "liberty",
	}},
	{GET, "/test/example/:var1/path/:var2", "/test/example/foobar/path/barbaz", map[string]string{
		"var1": "foobar",
		"var2": "barbaz",
	}},
}

func TestRouteMatch(t *testing.T) {
	for _, testroute := range testRoutes {
		router := newRouter()
		mux := http.NewServeMux()

		ctx := ctxPool.Get().(*Context)
		ctx.Reset()

		if err := router.Get(testroute.pattern, mux); err != nil {
			t.Error(err)
		}
		match, err := router.match(GET, testroute.path, ctx)
		if match == nil || err != nil {
			t.Errorf("bad search: %s", err)
			t.Errorf("pattern registered: %s", testroute.pattern)
			t.Errorf("path tested: %s", testroute.path)
		}

		if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
			t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
		}

		ctxPool.Put(ctx)
	}
}

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
		return router.Get(key, sg)
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
		return router.Get(key, sg)
	})

	w, req := httpWriterRequest("/user/subscriptions/graham/liberty")

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
