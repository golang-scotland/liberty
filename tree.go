package liberty

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// Tree is a ternary search trie used to map URL paths as application or public
// API routes (with or without parameters).
type Tree struct {
	root   *node
	router *Router
}

type node struct {
	lt         *node
	eq         *node
	gt         *node
	v          byte
	handlers   mHandlers
	varName    string
	varPattern *pattern
}

func (n *node) String() string {
	return fmt.Sprintf(
		"[value: %s, varName: %s, handlers: %#t]",
		string(n.v),
		n.varName,
		n.handlers,
	)
}

func (t *Tree) handle(nd *node, pattern *pattern, index int) *node {
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

func (t *Tree) Match(method method, path string, ctx *Context) http.Handler {
	l := len(path)
	n := t.root
	i := 0
	var match int
	var c byte

	for i < l {
		c = path[i]
		switch {
		default:
			n = n.eq
			i++
		case c == '/' && n.eq != nil && (n.eq.v == ':' || n.eq.v == '*'):
			// find and add the variable value to our context
			match = i + 1
			for match < l && path[match] != '/' {
				match++
			}
			ctx.Params.Add(n.eq.varName, path[i+1:match])

			// now we need to skip a few nodes and find the location in the tree
			// where the current variable name ends
			nextSegment := strings.Index(path[i+1:], "/")
			lastNode := nextSegment == -1 || n.eq.v == '*'
			i = i + 1 + nextSegment

			var sn *node
			var sc byte
			sn = n.eq

			searchPath := string(sn.v) + sn.varName
			if !lastNode && sn.v != '*' {
				searchPath = searchPath + "/"
			}

			sl := len(searchPath)
			si := 0
			for si < sl {
				sc = searchPath[si]
				switch {
				default:
					sn = sn.eq
					si++
				case sc < sn.v:
					sn = sn.lt
				case sc > sn.v:
					sn = sn.gt
				case si == sl-1:
					if lastNode {
						return sn.handlers[method]
					}
					n = sn
				}
			}
			/*
				if  {
					return nodeAfterVarName(n.eq, true).handlers[method]
				}
				n = nodeAfterVarName(n.eq, false)
			*/

			continue
		case n == nil || n.v == 0x0:
			return t.router.NotFound
		case c < n.v:
			n = n.lt
		case c > n.v:
			n = n.gt
		case i == l-1:
			/*
				if handler, ok := n.handlers[method]; ok {
					return handler
				}
			*/
			return n.handlers[method]
		}
	}

	return t.router.NotFound
}

func nodeAfterVarName(n *node, lastNode bool) *node {
	var path string
	var c byte
	var l, i int

	path = string(n.v) + n.varName
	if !lastNode && n.v != '*' {
		path = path + "/"
	}
	l = len(path)
	i = 0

	for i < l {
		c = path[i]
		switch {
		default:
			n = n.eq
			i++
		case c < n.v:
			n = n.lt
		case c > n.v:
			n = n.gt
		case i == l-1:
			return n
		}
	}

	return nil
}

func (t *Tree) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context().Value(CtxKey).(*Context)

	method, ok := methods[r.Method]
	if !ok {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	t.Match(method, r.URL.Path, ctx).ServeHTTP(w, r)

	ctxPool.Put(ctx)
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
