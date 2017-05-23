package liberty

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// tree is a ternary search trie used to map URL paths as application or public
// API routes (with or without parameters).
type tree struct {
	root   *node
	router *Router
}

type node struct {
	v          byte
	lt         *node
	eq         *node
	gt         *node
	varPattern *pattern
	handlers   mHandlers
	varName    string
}

func (n *node) String() string {
	return fmt.Sprintf(
		"[value: %s, varName: %s, handlers: %#t]",
		string(n.v),
		n.varName,
		n.handlers,
	)
}

func (t *tree) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := ctxPool.Get().(*Context)
	ctx.Reset()
	r = r.WithContext(context.WithValue(r.Context(), CtxKey, ctx))

	method, ok := methods[r.Method]
	if !ok {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	t.match(method, r.URL.Path, ctx).ServeHTTP(w, r)

	ctxPool.Put(ctx)
}

func (t *tree) handle(nd *node, pattern *pattern, index int) *node {
	v := pattern.str[index]

	if nd == nil {
		nd = &node{v: v}
	}

	varName, ok := pattern.varNameAt(index)
	if ok {
		nd.varName = varName
		nd.varPattern = pattern
	}

	if v < nd.v {
		nd.lt = t.handle(nd.lt, pattern, index)
	} else if v > nd.v {
		nd.gt = t.handle(nd.gt, pattern, index)
	} else if index < (len(pattern.str) - 1) {
		nd.eq = t.handle(nd.eq, pattern, index+1)
	} else {
		if nd.handlers == nil {
			nd.handlers = make(mHandlers, 0)
		}
		nd.handlers[pattern.method] = pattern.handler
	}

	return nd
}

func (t *tree) match(method method, path string, ctx *Context) http.Handler {
	var i int
	var match int
	var c byte

	n := t.root
	l := len(path)

	for i < l {
		c = path[i]
		switch {
		default:
			n = n.eq
			i++
		case c == '/' && n.eq != nil && (n.eq.v == ':' || n.eq.v == '*'):
			match = i + 1
			for match < l && path[match] != '/' {
				match++
			}
			ctx.Params.Add(n.eq.varName, path[i+1:match])

			nextSegment := strings.IndexByte(path[i+1:], '/')
			lastNode := nextSegment == -1 || n.eq.v == '*'
			i = i + 1 + nextSegment

			n = n.eq
			var sc byte
			var si int

			searchPath := string(n.v) + n.varName
			if !lastNode { // && n.v != '*' {
				searchPath = searchPath + "/"
			}
			sl := len(searchPath)

			for si < sl {
				sc = searchPath[si]
				switch {
				default:
					n = n.eq
					si++
				case sc < n.v:
					n = n.lt
				case sc > n.v:
					n = n.gt
				case si == sl-1:
					si++
				}
			}

			if lastNode {
				return n.handlers[method]
			}

			continue
		case n == nil || n.v == 0x0:
			return t.router.NotFound
		case c < n.v:
			n = n.lt
		case c > n.v:
			n = n.gt
		case i == l-1:
			return n.handlers[method]
		}
	}

	return t.router.NotFound
}

var ErrMethodNotAllowed = errors.New("Method verb for this routing pattern is not registered.")
var ErrNoRoute = errors.New("This route cannot be matched.")

/*
func (t *node) longestPrefix(mthd method, key string, ctx *Context) (http.Handler, error) {
	if len(key) < 1 {
		return nil, ErrNoRoute
	}

	length := prefix(t, key, 0)

	return t.match(mthd, key[0:length], ctx)
}
*/

func prefix(t *node, key string, index int) int {
	if index == len(key) || t == nil {
		return 0
	}

	length := 0
	recLen := 0
	v := key[index]

	if v < t.v {
		recLen = prefix(t.lt, key, index)
	} else if v > t.v {
		recLen = prefix(t.gt, key, index)
	} else {
		if t.v != 0x0 {
			length = index + 1
		}
		recLen = prefix(t.eq, key, index+1)
	}
	if length > recLen {
		return length
	}

	return recLen
}
