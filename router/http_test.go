package router

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"testing"
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

func newServerGroup() *ServerGroup {
	mux := http.NewServeMux()
	return &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux}, handler: mux}}}
}

func TestRouteMatch(t *testing.T) {

	router := NewHTTPRouter()
	sg := newServerGroup()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/example/path", sg); err != nil {
		t.Error(err)
	}
	match := router.match("/test/example/path", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", sg.servers[0].handler) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", sg, match)
	}

	ctxPool.Put(ctx)
}

func TestMatchLastVar(t *testing.T) {

	router := NewHTTPRouter()
	sg := newServerGroup()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/:var1", sg); err != nil {
		t.Error(err)
	}

	match := router.match("/test/foo", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", sg.servers[0].handler) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", sg, match)
	}

	ctxPool.Put(ctx)
}

func TestRouteMatchOneVar(t *testing.T) {

	router := NewHTTPRouter()
	sg := newServerGroup()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/:varone/bar", sg); err != nil {
		t.Error(err)
	}

	match := router.match("/test/foo/bar", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", sg.servers[0].handler) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", sg, match)
	}

	ctxPool.Put(ctx)
}

func TestRouteMatchTwoVar(t *testing.T) {

	router := NewHTTPRouter()
	sg := newServerGroup()
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	if err := router.Handle("/test/example/:var1/path/:var2", sg); err != nil {
		t.Error(err)
	}

	match := router.match("/test/example/foobar/path/barbaz", ctx)
	if match == nil {
		t.Errorf("bad search:")
	}

	if fmt.Sprintf("%p", sg.servers[0].handler) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", sg, match)
	}

	ctxPool.Put(ctx)
}

func TestRouteMatchLongest(t *testing.T) {

	router := &HTTPRouter{}
	mux := http.NewServeMux()
	sg := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux}, handler: mux}}}
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	router.Handle("http://www.example.com/*", sg)
	match := router.match("http://www.example.com/foo/", ctx)
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
	router := &HTTPRouter{}
	sg := newServerGroup()
	loadGithubApi(func(key string) error {
		return router.Handle(key, sg)
	})

	b.ReportAllocs()
	b.ResetTimer()

	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	for n := 0; n < b.N; n++ {
		_ = router.match("/user/repos", ctx)
	}

	ctxPool.Put(ctx)
}

func BenchmarkTreeGetVar1000(b *testing.B) {
	router := &HTTPRouter{}
	sg := newServerGroup()

	loadGithubApi(func(key string) error {
		return router.Handle(key, sg)
	})

	b.ReportAllocs()
	b.ResetTimer()

	ctx := ctxPool.Get().(*Context)
	ctx.Reset()

	for n := 0; n < b.N; n++ {
		_ = router.match("/users/graham/gists", ctx)
	}

	ctxPool.Put(ctx)
}
