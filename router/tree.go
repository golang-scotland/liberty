package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

type Tree struct {
	root   *node
	router *Router
}

// a ternary search tree/trie (tree for the avoidance of pronunciation doubt)
type node struct {
	lt         *node
	eq         *node
	gt         *node
	v          byte
	handlers   mHandlers
	varName    string
	varEnd     *node
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
	var nextString int
	var match int
	var matchedVal string
	var c byte

	for i < l {
		c = path[i]
		switch {
		case c == '/' && n.eq != nil && (n.eq.v == ':' || n.eq.v == '*'):
			for match = i + 1; match < len(path) && path[match] != '/'; match++ {
				if match == len(path)-1 {
					matchedVal = path[i+1 : match+1]

				} else {
					matchedVal = path[i+1 : match+1]
				}

			}

			ctx.Params.Add(n.eq.varName, matchedVal)

			nextString = strings.Index(path[i+1:], "/")
			if nextString == -1 || n.eq.v == '*' {
				return nodeAfterVarName(n.eq, true).handlers[method]
			}
			i++
			i = i + nextString
			n = nodeAfterVarName(n.eq, false)

			continue
		case n == nil || n.v == 0x0:
			return nil
		case c < n.v:
			n = n.lt
		case c > n.v:
			n = n.gt
		case i == l-1:
			if handler, ok := n.handlers[method]; ok {
				return handler
			}
		default:
			n = n.eq
			i++
		}
	}

	return nil
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

		if c < n.v {
			n = n.lt
		} else if c > n.v {
			n = n.gt
		} else if i == l-1 {
			return n
		} else {
			n = n.eq
			i++
		}
	}

	return nil
}

func (t *Tree) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context().Value(CtxKey).(*Context)
	var handler http.Handler
	//var err error
	var method method
	var notAllowed = func() {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write(nil)
	}

	method, ok := methods[r.Method]
	if !ok {
		notAllowed()
		return
	}

	handler = t.Match(method, r.URL.Path, ctx)
	if handler == nil {
		t.router.NotFound.ServeHTTP(w, r)
		return
	}

	handler.ServeHTTP(w, r)
	ctxPool.Put(ctx)
}

var ErrMethodNotAllowed = errors.New("Method verb for this routing pattern is not registered.")
var ErrNoRoute = errors.New("This route cannot be matched.")

func printTraversal(t *node) {
	printTraversalAux(t, []byte(""))
}

func printTraversalAux(t *node, prefix []byte) {
	if t != nil {

		/* Start normal in-order traversal.
		   This prints all words that come alphabetically before the words rooted here.*/
		printTraversalAux(t.lt, prefix)

		/* List all words starting with the current character. */
		if t.handlers != nil {
			endChars := append(prefix, t.v)
			fmt.Println(string(endChars))
		}

		if t.eq != nil {
			eqChars := append(prefix, t.v)
			printTraversalAux(t.eq, eqChars)
		}

		/* Finish the in-order traversal by listing all words that come after this word.*/
		if t.gt != nil {
			gtChars := append(prefix, t.gt.v)
			printTraversalAux(t.gt, gtChars)
		}

	}
}

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
