// Package router implements a ternary search tree based HTTP router.
package liberty

import (
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
// satisfies the standard libray http.Handler interface.
type Router struct {
	tree     *tree
	chain    *middleware.Chain
	Handler  http.Handler
	NotFound http.Handler
}

func NewRouter() *Router {
	r := &Router{tree: &tree{}}
	r.tree.router = r

	return r
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt.Handler.ServeHTTP(w, r)
}

func (rt *Router) Use(handlers []middleware.Chainable) {
	rt.chain = middleware.NewChain(handlers...)
	rt.Handler = rt.chain.Link(rt)
}

func (rt *Router) handle(method method, path string, handler http.Handler) error {
	if rt.tree == nil {
		rt.tree = &tree{router: rt}
	}

	if rt.Handler == nil {
		rt.Handler = rt.tree
	}

	if rt.NotFound == nil {
		rt.NotFound = http.HandlerFunc(http.NotFound)
	}

	pat := NewPattern(method, path, handler)
	rt.tree.root = rt.tree.handle(rt.tree.root, pat, 0)

	return nil
}

func (rt *Router) Get(path string, handler http.Handler) error {
	return rt.handle(GET, path, handler)
}

func (rt *Router) Post(path string, handler http.Handler) error {
	return rt.handle(POST, path, handler)
}

func (rt *Router) Put(path string, handler http.Handler) error {
	return rt.handle(PUT, path, handler)
}

func (rt *Router) Patch(path string, handler http.Handler) error {
	return rt.handle(PATCH, path, handler)
}

func (rt *Router) Delete(path string, handler http.Handler) error {
	return rt.handle(DELETE, path, handler)
}

func (rt *Router) match(method method, path string, ctx *Context) http.Handler {
	return rt.tree.match(method, path, ctx)
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
