// Package router implements a ternary search tree based HTTP router. The main
// focus of this package is to support using a single HTTP request muxer that
// multiple HTTP servers.
package router

import (
	"context"
	"fmt"
	"net/http"

	"golang.scot/liberty/middleware"
)

// HTTPRouter is a ternary search tree/trie based HTTP request router.
type HTTPRouter struct {
	tree    *tree
	chain   *middleware.Chain
	handler http.Handler
}

func NewHTTPRouter() *HTTPRouter {
	return &HTTPRouter{tree: &tree{
		handlers: make(mHandlers, 0),
	}}
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

func (h *HTTPRouter) Delete(path string, handler http.Handler) error {
	return h.handle(DELETE, path, handler)
}

func (h *HTTPRouter) Patch(path string, handler http.Handler) error {
	return h.handle(PATCH, path, handler)
}

func (h *HTTPRouter) Put(path string, handler http.Handler) error {
	return h.handle(PUT, path, handler)
}
