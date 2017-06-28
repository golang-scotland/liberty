// +build gofuzz

package liberty

import (
	"bufio"
	"context"
	"net/http"
	"strings"
)

func Fuzz(data []byte) int {
	s := string(data)
	r0 := s[:len(s)/3]
	r1 := s[len(s)/3 : len(s)/3*2]
	reqs := s[len(s)/3*2:]

	handler := handleFuzz()
	router := NewRouter()
	router.Get(r0, handler)
	router.Get(r1, handler)

	if req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(reqs))); err == nil {
		ctx := ctxPool.Get().(*Context)
		req = req.WithContext(context.WithValue(req.Context(), CtxKey, ctx))
		ctx.Reset()

		h := router.tree.match(GET, req.URL.Path, ctx)
		if h == nil {
			panic("no handler returned")
		}
		ctxPool.Put(ctx)
	}
	return 0
}

func handleFuzz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}
