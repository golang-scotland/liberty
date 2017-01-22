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

func newRouter() *HTTPRouter {
	router := NewHTTPRouter()
	/*router.Use(
		[]middleware.Chainable{&middleware.HelloWorld{}},
	)
	*/

	return router
}

var routerTests = []struct {
	pattern string
	path    string
}{
	{"/", "/"},
	{"/test/example/path", "/test/example/path"},
	{"/:a/:b/:c/:d/:e", "/test/test/test/test/test"},
	{"/foo/:bar/baz", "/foo/:bar/baz"},
	{"/repos/:owner/:repo/stargazers", "/repos/graham/liberty/stargazers"},
	{"/test/:var1", "/test/foo"},
	{"/test/:varone/bar", "/test/foo/bar"},
	{"/test/example/:var1/path/:var2", "/test/example/foobar/path/barbaz"},
	{"/wildcard/pattern/*", "/wildcard/pattern/matches/this"},
}

func TestRouteMatch(t *testing.T) {
	for _, rt := range routerTests {

		router := newRouter()
		mux := http.NewServeMux()
		ctx := ctxPool.Get().(*Context)
		ctx.Reset()

		if err := router.Get(rt.pattern, mux); err != nil {
			t.Error(err)
		}

		match, _ := router.tree.match(GET, rt.path, ctx)

		if match == nil {
			t.Errorf("bad search:")
		}

		if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
			t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
		}

		ctxPool.Put(ctx)
	}
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
	match, _ := router.Getc("http://www.example.com/foo/bar")
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

	w, req := httpWriterRequest("/user/repos")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		router.ServeHTTP(w, req)
	}
}
