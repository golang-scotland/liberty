// Package router implements a ternary search tree based HTTP router.
package router

import (
	"context"
	"net/http"
	"strings"

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
		DELETE:  "DELETE",
	}

	return methods[m]
}

type mHandlers map[method]http.Handler

// Router is a ternary search tree based HTTP request router. Router
// satifsies the standard libray http.Handler interface.
type Router struct {
	tree     *Tree
	chain    *middleware.Chain
	handler  http.Handler
	NotFound http.Handler
}

func NewRouter() *Router {
	r := &Router{tree: &Tree{}}
	r.tree.router = r

	return r
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
		h.tree = &Tree{router: h}
	}

	if h.handler == nil {
		h.handler = h.tree
	}

	if h.NotFound == nil {
		h.NotFound = http.HandlerFunc(http.NotFound)
	}

	pat := NewPattern(method, path, handler)
	h.tree.root = h.tree.handle(h.tree.root, pat, 0)

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

func (h *Router) Patch(path string, handler http.Handler) error {
	return h.handle(PATCH, path, handler)
}

func (h *Router) Delete(path string, handler http.Handler) error {
	return h.handle(DELETE, path, handler)
}

func (h *Router) match(method method, path string, ctx *Context) http.Handler {
	return h.tree.Match(method, path, ctx)
}

type patternVariable struct {
	name       string
	typ        string
	endLoc     int
	endLocNode *node
}

type pattern struct {
	str      string
	method   method
	varCount int
	locs     map[int]*patternVariable
	handler  http.Handler
}

func NewPattern(method method, pat string, handler http.Handler) *pattern {
	p := &pattern{
		str:      pat,
		method:   method,
		varCount: 0,
		locs:     make(map[int]*patternVariable, 0),
		handler:  handler,
	}
	p.setVarCount()

	return p
}

func (p *pattern) setVarCount() {
	for i := 0; i < len(p.str); i++ {
		if i > 0 && i < len(p.str)-1 && (p.str[i] == ':' || p.str[i] == '*') {
			splits := strings.Split(p.str[i+1:], "/")
			p.varCount++
			p.locs[i] = &patternVariable{
				name:   splits[0],
				typ:    "default",
				endLoc: i + len(splits[0]),
			}
		}
	}
}

func (p *pattern) varNameAt(i int) (string, bool) {
	for j, variable := range p.locs {
		if i == j {
			return variable.name, true
		}
	}

	return "", false
}
