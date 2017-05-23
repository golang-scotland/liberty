// Package router implements a ternary search tree based HTTP router.
package liberty

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

// Router is a ternary search tree based HTTP request router. Router satisfies
// the standard libray http.Handler interface.
type Router struct {
	tree     *tree
	chain    *middleware.Chain
	handler  http.Handler
	NotFound http.Handler
}

// NewRouter returns an HTTP request router ready for immediate use
func NewRouter() *Router {
	r := &Router{tree: &tree{}}
	r.tree.router = r

	return r
}

// ServeHTTP will first try to route the request through any chained handlers
// and then it will fallback to route matching against the router trie
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	r = r.WithContext(context.WithValue(r.Context(), CtxKey, ctx))

	method, ok := methods[r.Method]
	if !ok {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if rt.handler != nil {
		rt.handler.ServeHTTP(w, r)
	} else {
		rt.tree.match(method, r.URL.Path, ctx).ServeHTTP(w, r)
	}

	ctxPool.Put(ctx)
}

// Use registers a chain of wrapped http.Handlers, the last handler in the chain
// is always this router itself.
func (rt *Router) Use(handlers []middleware.Chainable) {
	rt.chain = middleware.NewChain(handlers...)
	rt.handler = rt.chain.Link(rt)
}

func (rt *Router) handle(method method, path string, handler http.Handler) error {
	if rt.tree == nil {
		rt.tree = &tree{router: rt}
	}

	if rt.NotFound == nil {
		rt.NotFound = http.HandlerFunc(http.NotFound)
	}

	pat := newPattern(method, path, handler)
	rt.tree.root = rt.tree.handle(rt.tree.root, pat, 0)

	return nil
}

// Get registers a URL routing path and handler for the GET HTTP verb
func (rt *Router) Get(path string, handler http.Handler) error {
	return rt.handle(GET, path, handler)
}

// Post registers a URL routing path and handler for the POST HTTP verb
func (rt *Router) Post(path string, handler http.Handler) error {
	return rt.handle(POST, path, handler)
}

// Put registers a URL routing path and handler for PUT HTTP verb
func (rt *Router) Put(path string, handler http.Handler) error {
	return rt.handle(PUT, path, handler)
}

// Patch registers a URL routing path and handler for PATCH HTTP verb
func (rt *Router) Patch(path string, handler http.Handler) error {
	return rt.handle(PATCH, path, handler)
}

// Delete registers a URL routing path and handler for DELETE HTTP verb
func (rt *Router) Delete(path string, handler http.Handler) error {
	return rt.handle(DELETE, path, handler)
}

// TODO make these setters noop on error and return last error
func (rt *Router) All(path string, handler http.Handler) error {
	rt.handle(GET, path, handler)
	rt.handle(POST, path, handler)
	rt.handle(PUT, path, handler)
	rt.handle(PATCH, path, handler)
	rt.handle(DELETE, path, handler)
	return nil
}

type patternVariable struct {
	name string
}

type pattern struct {
	str      string
	method   method
	varCount int
	locs     map[int]*patternVariable
	handler  http.Handler
}

func newPattern(method method, pat string, handler http.Handler) *pattern {
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
				name: splits[0],
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
