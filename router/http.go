// Package router implements a ternary search tree based HTTP router. The main
// focus of this package is to support using a single HTTP request muxer that
// multiple HTTP servers.
package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// HTTPRouter is a ternary search tree based HTTP request router. HTTPRouter
// satifsies the standard libray http.Handler interface.
type HTTPRouter struct {
	tree *tree
}

func NewHTTPRouter() *HTTPRouter {
	return &HTTPRouter{tree: &tree{}}
}

func (h *HTTPRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	r = r.WithContext(context.WithValue(r.Context(), CtxKey, ctx))

	if handler := h.match(r.URL.Path, ctx); handler != nil {
		handler.ServeHTTP(w, r)
		fmt.Println(ctx.Params)
		ctxPool.Put(ctx)
		return
	}

	http.NotFound(w, r)
	ctxPool.Put(ctx)
}

func (h *HTTPRouter) Handle(path string, serverGroup *ServerGroup) error {
	if h.tree == nil {
		h.tree = &tree{}
	}
	if err := h.tree.handle(path, serverGroup); err != nil {
		fmt.Printf("could not register HotPath '%s' - %s", path, err)
		return err
	}
	return nil
}

func (h *HTTPRouter) match(path string, ctx *Context) http.Handler {
	var sg *ServerGroup
	if sg = h.tree.match(path, ctx); sg == nil {
		if strings.HasSuffix(path, "*") {
			if sg = h.longestPrefix(path[:len(path)-1], ctx); sg != nil {
				return sg.leastUsed()
			}
		}
		return nil
	}

	return sg.leastUsed()
}

// a ternary search trie (tree for the avoidance of pronunciation doubt)
type tree struct {
	lt      *tree
	eq      *tree
	gt      *tree
	v       byte
	sg      *ServerGroup
	varName string
}

func (t *tree) String() string {
	return fmt.Sprintf("[value: %s, t.sg: %v, t.varName: %s]", string(t.v), t.sg, t.varName)
}

func (t *tree) handle(path string, sg *ServerGroup) error {
	if sg == nil {
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
		case v == '/' && path[i+1] == ':':
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
				t.sg = sg
				return nil
			}

		case v < t.v:
			t = t.lt

		case v > t.v:
			t = t.gt

		case i == lastChar:
			t.sg = sg
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

// match the route
func (t *tree) match(path string, ctx *Context) *ServerGroup {
	l := len(path)
	lastChar := l - 1

	for i := 0; i < l; {
		v := path[i]

		switch {
		case t.v == 0x0:
			return nil

		case v == '/' && t.eq.v == '*':
			return t.eq.sg
		case v == '/' && t.gt.v == ':':
			val, end := routeVarGet(i, path)
			ctx.Params.Add(t.gt.varName, val)
			if end >= lastChar {
				return t.gt.sg
			}
			i = end
			t = t.gt.lt

		case v > t.v:
			t = t.gt

		case v < t.v:
			t = t.lt

		case i == lastChar && t.sg != nil:
			return t.sg

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

func (h *HTTPRouter) longestPrefix(key string, ctx *Context) *ServerGroup {
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
