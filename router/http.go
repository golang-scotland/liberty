// Package router implements a ternary search tree based HTTP router. The main
// focus of this package is to support using a single HTTP request muxer that
// multiple HTTP servers.
package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.scot/liberty/middleware"
)

// HTTPRouter is a ternary search tree based HTTP request router. HTTPRouter
// satifsies the standard libray http.Handler interface.
type HTTPRouter struct {
	tree    *tree
	chain   *middleware.Chain
	handler http.Handler
}

func NewHTTPRouter() *HTTPRouter {
	return &HTTPRouter{tree: &tree{}}
}

func (h *HTTPRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	r = r.WithContext(context.WithValue(r.Context(), CtxKey, ctx))
	h.handler.ServeHTTP(w, r)
	ctxPool.Put(ctx)
}

func (h *HTTPRouter) Use(handlers []middleware.Chainable) {
	if handlers == nil {
		h.handler = h.tree
		return
	}
	h.chain = middleware.NewChain(handlers...)
	h.handler = h.chain.Link(h.tree)
}

// Handle registers a routing path and http.Handler
func (h *HTTPRouter) Handle(path string, handler http.Handler) error {
	if h.tree == nil {
		h.tree = &tree{}
	}
	if err := h.tree.handle(path, handler); err != nil {
		fmt.Printf("could not register HotPath '%s' - %s", path, err)
		return err
	}
	return nil
}

func (h *HTTPRouter) match(path string, ctx *Context) http.Handler {
	var handler http.Handler
	if handler = h.tree.match(path, ctx); handler == nil {
		if strings.HasSuffix(path, "*") {
			if handler = h.longestPrefix(path[:len(path)-1], ctx); handler != nil {
				return handler
			}
		}
		return nil
	}

	return handler
}

// a ternary search trie (tree for the avoidance of pronunciation doubt)
type tree struct {
	lt      *tree
	eq      *tree
	gt      *tree
	v       byte
	h       http.Handler
	varName string
}

func (t *tree) String() string {
	return fmt.Sprintf("[value: %s, t.h: %v, t.varName: %s]", string(t.v), t.h, t.varName)
}

func (t *tree) handle(path string, handler http.Handler) error {
	if handler == nil {
		panic("nil group")
	}
	l := len(path)
	lastChar := l - 1

	var err error
	var varEnd int

	for i := 0; i < l; {
		v := path[i]
		if t.v == 0x0 {
			t.v = v
			t.lt = &tree{}
			t.eq = &tree{}
			t.gt = &tree{}
		}

		switch {
		case v == '/' && i != lastChar && path[i+1] == ':':
			if varEnd, err = routeVarEnd(i, path); err != nil {
				return err
			}
			t = t.gt
			t.v = ':'
			t.varName = string(path[i+2 : varEnd])
			t.lt = &tree{}
			t.eq = &tree{}
			t.gt = &tree{}

			i = varEnd
			if varEnd > lastChar {
				t.h = handler
				return nil
			}

		case v < t.v:
			t = t.lt

		case v > t.v:
			t = t.gt

		case i == lastChar:
			t.h = handler
			return nil

		default:
			t = t.eq
			i++
		}
	}

	return fmt.Errorf("unable to insert handler for key '%s'", path)
}

func numParams(path string) (n uint) {
	for i := 0; i < len(path); i++ {
		if path[i] != ':' && path[i] != '*' {
			continue
		}
		n++
	}

	return n
}

func routeVarEnd(start int, path string) (end int, err error) {
	end = start + 2
	routeLen := len(path)
	for end < routeLen && path[end] != '/' {
		switch path[end] {
		case ':':
			return 0, fmt.Errorf("invalid character '%s' in variable name", ":")
		default:
			end++
		}
	}

	return end, nil
}

func (t *tree) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context().Value(CtxKey).(*Context)

	h := t.match(r.URL.Path, ctx)
	if h == nil {
		fmt.Println(r.URL.Path)
		fmt.Println(ctx)
		panic("no MATCH")
		http.NotFound(w, r)
	}

	h.ServeHTTP(w, r)
}

// match the route
func (t *tree) match(path string, ctx *Context) http.Handler {
	var end int

	for i := 0; i < len(path); {
		v := path[i]

		switch {
		case t.v == 0x0:
			return nil

		case v == '/' && t.eq.v == '*':
			return t.eq.h
		case v == '/' && t.gt.v == ':':
			for end = i + 1; end < len(path) && path[end] != '/'; end++ {
				if path[end] == '/' {
					ctx.Params.Add(t.gt.varName, path[i:end])
				}
			}

			if end >= len(path)-1 {
				return t.gt.h
			}
			i = end
			t = t.gt.lt

		case v > t.v:
			t = t.gt

		case v < t.v:
			t = t.lt

		case i == len(path)-1 && t.h != nil:
			return t.h

		default:
			t = t.eq
			i++
		}
	}

	return nil
}

func routeVarGet(varStart int, path string) (val string, end int) {
	end = varStart + 1
	routeLen := len(path)
	for end < routeLen && path[end] != '/' {
		switch path[end] {
		case '/':
			return path[varStart:end], end
		default:
			end++
		}
	}

	return path[varStart+1 : end], end
}

func (h *HTTPRouter) longestPrefix(key string, ctx *Context) http.Handler {
	if len(key) < 1 {
		return nil
	}

	length := h.prefix(h.tree, key, 0)

	return h.tree.match(key[0:length], ctx)
}

func (h *HTTPRouter) prefix(n *tree, key string, index int) int {
	if index == len(key) || n == nil {
		return 0
	}

	length := 0
	recLen := 0
	v := key[index]

	if v < n.v {
		recLen = h.prefix(n.lt, key, index)
	} else if v > n.v {
		recLen = h.prefix(n.gt, key, index)
	} else {
		if n.v != 0x0 {
			length = index + 1
		}
		recLen = h.prefix(n.eq, key, index+1)
	}
	if length > recLen {
		return length
	}
	return recLen
}
