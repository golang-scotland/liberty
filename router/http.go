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
	h.chain = middleware.NewChain(handlers...)
	h.handler = h.chain.Link(h.tree)
}

func (h *HTTPRouter) handle(method method, path string, handler http.Handler) error {
	if h.tree == nil {
		h.tree = &tree{}
	}

	if h.handler == nil {
		h.handler = h.tree
	}

	if h.tree.handlers == nil {
		h.tree.handlers = make(mHandlers, 0)
	}

	if err := h.tree.handle(method, path, handler); err != nil {
		fmt.Printf("could not register HotPath '%s' - %s", path, err)
		return err
	}
	return nil
}

func (h *HTTPRouter) Get(path string, handler http.Handler) error {
	return h.handle(GET, path, handler)
}

func (h *HTTPRouter) Post(path string, handler http.Handler) error {
	return h.handle(POST, path, handler)
}

func (h *HTTPRouter) Put(path string, handler http.Handler) error {
	return h.handle(PUT, path, handler)
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
