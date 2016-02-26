package router

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"testing"
)

func TestExactMatch(t *testing.T) {
	router := &HttpRouter{}
	mux := http.NewServeMux()
	sg := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux}, handler: mux}}}
	if err := router.put("http://www.example.com", sg); err != nil {
		t.Errorf("insertion error: foo")
	}
	if match := router.Get("http://www.example.com"); match == nil {
		t.Errorf("bad search: foo")
	}
}

func TestRouteMatch(t *testing.T) {

	router := &HttpRouter{}
	mux := http.NewServeMux()
	sg := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux}, handler: mux}}}
	router.put("http://www.example.com", sg)
	match := router.Get("http://www.example.com/foo/")
	if match == nil {
		t.Errorf("bad search: foo")
	}
	if fmt.Sprintf("%p", mux) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: - h: %#v,  match: %#v", mux, match)
	}
}

func TestLongesPrefixtMatch(t *testing.T) {
	router := &HttpRouter{}
	mux1 := http.NewServeMux()
	mux2 := http.NewServeMux()
	h1 := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux1}, handler: mux1}}}
	h2 := &ServerGroup{servers: []*server{{s: &http.Server{Handler: mux2}, handler: mux2}}}
	router.put("http://www.example.com/", h1)
	router.put("http://www.example.com/foo/", h2)
	match := router.Get("http://www.example.com/foo/bar")
	if match == nil {
		t.Errorf("bad search: no match")
	}
	if fmt.Sprintf("%p", mux2) != fmt.Sprintf("%p", match) {
		t.Errorf("address mismatch: h2: %#v,  match: %#v", mux2, match)
	}
}

/*func BenchmarkTreePut(b *testing.B) {
	router := &HttpRouter{}
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
}
*/
func valuesForBenchmark(numValues int, cb func(string)) {
	rand.Seed(42)
	for n := 0; n < numValues; n++ {
		key := []rune{}
		if n == int(math.Floor(float64(numValues/2.0))) {
			key = []rune("www.match.com/api/path")
		} else {
			for n := 0; n < rand.Intn(1000); n++ {
				key = append(key, rune(rand.Intn(94)+32))
			}
		}
		//fmt.Println(string(key))
		cb(string(key))
	}
}

func BenchmarkTreeGet100(b *testing.B) {
	router := &HttpRouter{}
	sg := &ServerGroup{servers: []*server{{s: &http.Server{}}}}
	valuesForBenchmark(100, func(key string) { router.Put(key, sg) })

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		//_ = router.Get("www.example.com/foo")
		_ = router.Get("www.match.com/api/path")
	}
}

func BenchmarkTreeGetC100(b *testing.B) {
	router := &HttpRouter{}
	sg := &ServerGroup{servers: []*server{{s: &http.Server{}}}}
	valuesForBenchmark(100, func(key string) { router.Put(key, sg) })

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		//_ = router.Get("www.example.com/foo")
		_ = router.Getc("www.match.com/api/path")
	}
}
func BenchmarkMapGet100(b *testing.B) {
	hash := make(map[string]http.Handler)
	valuesForBenchmark(100, func(key string) { hash[key] = http.NewServeMux() })
	/*var needle string
	for key, _ := range hash {
		needle = key
		break
	}*/

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		//_ = hash["www.example.com/foo"]
		_ = hash["www.match.com/api/path"]
	}
}
