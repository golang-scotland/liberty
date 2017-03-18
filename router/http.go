// Package router implements a ternary search tree based HTTP router.
package router

import (
	"context"
	"fmt"
	"net/http"

	"golang.scot/liberty/middleware"
)

type method int

const (
	GET = 1 << iota
	POST
	PUT
	PATCH
	OPTIONS
	HEAD
	RANGE
	DELETE
)

var methods = map[string]method{
	"GET":     GET,
	"POST":    POST,
	"PUT":     PUT,
	"PATCH":   PATCH,
	"OPTIONS": OPTIONS,
	"HEAD":    HEAD,
	"RANGE":   RANGE,
	"DELETE":  DELETE,
}

func (m method) String() string {
	var methods = map[method]string{
		GET:     "GET",
		POST:    "POST",
		PUT:     "PUT",
		PATCH:   "PATCH",
		OPTIONS: "OPTIONS",
		HEAD:    "HEAD",
		RANGE:   "RANGE",
	}

	return methods[m]
}

type mHandlers map[method]http.Handler

// Router is a ternary search tree based HTTP request router. Router
// satifsies the standard libray http.Handler interface.
type Router struct {
	tree    *tree
	chain   *middleware.Chain
	handler http.Handler
}

func NewHTTPRouter() *Router {
	return &Router{tree: &tree{}}
}

func (h *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	r = r.WithContext(context.WithValue(r.Context(), CtxKey, ctx))
	h.handler.ServeHTTP(w, r)
	ctxPool.Put(ctx)
}

func (h *Router) Use(handlers []middleware.Chainable) {
	h.chain = middleware.NewChain(handlers...)
	h.handler = h.chain.Link(h.tree)
}

func (h *Router) handle(method method, path string, handler http.Handler) error {
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

func (h *Router) Get(path string, handler http.Handler) error {
	return h.handle(GET, path, handler)
}

func (h *Router) Post(path string, handler http.Handler) error {
	return h.handle(POST, path, handler)
}

func (h *Router) Put(path string, handler http.Handler) error {
	return h.handle(PUT, path, handler)
}

func (h *Router) match(method method, path string, ctx *Context) (http.Handler, error) {
	return h.tree.match(method, path, ctx)
}
