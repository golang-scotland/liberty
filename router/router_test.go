package router

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pressly/chi"
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
	router := NewRouter()
	/*router.Use(
		[]middleware.Chainable{&middleware.HelloWorld{}},
	)
	*/

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

func TestSingleMatch(t *testing.T) {
	pattern := "/test/example/:var1/path/:var2"
	pattern2 := "/:ab/:bc/:cd/:de/:ef"
	pattern3 := "/test2/example/:foo/path/:bar"
	path := "/test/example/foobar/path/barbaz"

	router := newRouter()
	mux := http.NewServeMux()

	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Get(pattern, mux); err != nil {
		t.Error(err)
	}
	if err := router.Get(pattern2, mux); err != nil {
		t.Error(err)
	}
	if err := router.Get(pattern3, mux); err != nil {
		t.Error(err)
	}

	match := router.match(GET, path, ctx)
	if match == nil {
		t.Errorf("bad search: %s")
		t.Errorf("pattern registered: %s", pattern)
		t.Errorf("path tested: %s", path)
	}

	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}
	if ctx.Params.Get("var2") != "barbaz" {
		t.Errorf("variable mismatch - expected '%s', got '%s'", "foobar", ctx.Params.Get("var2"))
	}
	printTraversal(router.tree.root)
}

func TestGithub(t *testing.T) {
	r := newRouter()
	for _, route := range githubAPI {
		h := http.NewServeMux()

		ctx := ctxPool.Get().(*Context)
		ctx.Reset()

		var testErr = func(err error) {
			if err != nil {
				panic(err)
			}
		}

		switch route.method {
		case "GET":
			testErr(r.Get(route.path, h))
		case "POST":
			testErr(r.Post(route.path, h))
		case "PUT":
			testErr(r.Put(route.path, h))
		case "PATCH":
			testErr(r.Patch(route.path, h))
		case "DELETE":
			testErr(r.Delete(route.path, h))
		default:
			panic("Unknown HTTP method: " + route.method)
		}

		ctxPool.Put(ctx)
	}

	for _, route := range githubAPI {
		ctx := ctxPool.Get().(*Context)
		ctx.Reset()

		method, _ := methods[route.method]
		match := r.match(method, route.path, ctx)
		if match == nil {
			t.Errorf("path tested: %s", route.path)
		}

		ctxPool.Put(ctx)
	}
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
		match := router.match(GET, testroute.path, ctx)
		if match == nil {
			t.Errorf("bad search: %s")
			t.Errorf("pattern registered: %s", testroute.pattern)
			t.Errorf("path tested: %s", testroute.path)
		}

		for key, expected := range testroute.testMatches {
			if ctx.Params.Get(key) != expected {
				t.Errorf("bad value - expected %s, got %s", expected, ctx.Params.Get(key))
			}
		}

		if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
			t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
		}

		ctxPool.Put(ctx)
	}
}

func TestBuiltTreeMatch(t *testing.T) {
	router := newRouter()
	for _, testroute := range testRoutes {
		mux := http.NewServeMux()

		if err := router.Get(testroute.pattern, mux); err != nil {
			t.Error(err)
		}
	}

	printTraversal(router.tree.root)
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

	for n := 0; n < b.N; n++ {
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChiGet1000(b *testing.B) {
	router := chi.NewRouter()
	sg := newServerGroup()
	loadGithubApi(func(key string) error {
		router.Get(key, func(w http.ResponseWriter, r *http.Request) {
			sg.ServeHTTP(w, r)
		})
		return nil
	})

	w, req := httpWriterRequest("/user/repos")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		router.ServeHTTP(w, req)
	}
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

	for n := 0; n < b.N; n++ {
		router.ServeHTTP(w, req)
	}
}
