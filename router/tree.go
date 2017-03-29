package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

type Tree struct {
	root *node
}

func (t *Tree) TServeHTTP(w http.ResponseWriter, r *http.Request) {
	//t.root.ServeHTTP(w, r)
}

// a ternary search tree/trie (tree for the avoidance of pronunciation doubt)
type node struct {
	lt       *node
	eq       *node
	gt       *node
	v        byte
	handlers mHandlers
	varName  string
	varEnd   *node
}

func (n *node) String() string {
	return fmt.Sprintf(
		"[value: %s, t.varName: %s, handlers: %#t]",
		string(n.v),
		n.varName,
		n.handlers,
	)
}

var nodeVarStart *node

func (t *Tree) handleRecursive(nd *node, pattern *pattern, index int) *node {
	v := pattern.str[index]

	if nd == nil {
		nd = &node{v: v}
	}

	varName, ok := pattern.varNameAt(index)
	if ok {
		nodeVarStart = nd
		nd.varName = varName
	}

	if nd.v == '/' {
		nd.varEnd = nodeVarStart
	}

	if v < nd.v {
		nd.lt = t.handleRecursive(nd.lt, pattern, index)
	} else if v > nd.v {
		nd.gt = t.handleRecursive(nd.gt, pattern, index)
	} else if index < (len(pattern.str) - 1) {
		nd.eq = t.handleRecursive(nd.eq, pattern, index+1)
	} else {
		//fmt.Println("HANDLED", string(v))
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
	// ctx.Params.Add(t.gt.varName, matchedVal)
	for i < l {
		c = path[i]
		switch {
		case c == '/' && n.eq != nil && (n.eq.v == ':' || n.eq.v == '*'):
			//fmt.Println("VAR START")
			for match = i + 1; match < len(path) && path[match] != '/'; match++ {
				if match == len(path)-1 {
					matchedVal = path[i+1 : match+1]

				} else {
					matchedVal = path[i+1 : match+1]
				}

			}
			//splits := strings.Split(path[i+1:], "/")
			//val := splits[0]
			//fmt.Println("varName:", n.eq.varName, "value:", val)
			ctx.Params.Add(n.eq.varName, matchedVal)

			nextString = strings.Index(path[i+1:], "/")
			if nextString == -1 || n.eq.v == '*' {
				//fmt.Println("END", path[i:])
				return nodeAfterVarName(n.eq, true).handlers[method]
			}
			i++
			i = i + nextString
			//fmt.Println("MID", path[i:])
			n = nodeAfterVarName(n.eq, false)
			//n = n.varEnd

			continue
		case n == nil || n.v == 0x0:
			//fmt.Println("MATCH NONE")
			return nil
		case c < n.v:
			//fmt.Println("LT", string(c), n.lt, n.eq, n.gt)
			n = n.lt
		case c > n.v:
			//fmt.Println("GT", string(c), n.lt, n.eq, n.gt)
			n = n.gt
		case i == l-1:
			if handler, ok := n.handlers[method]; ok {
				//fmt.Println("MATCH", handler)
				return handler
			}
		default:
			//fmt.Println("EQ", string(c), n.lt, n.eq, n.gt)
			n = n.eq
			i++
		}
	}

	//fmt.Println("DERP")
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

// register a given handler for a pattern with a specific HTTP verb
func (t *node) handle(method method, pattern string, handler http.Handler) error {
	//fmt.Println("HANDLE", method, pattern, handler)
	if handler == nil {
		panic("Handler may not be nil")
	}
	l := len(pattern)
	lastChar := l - 1

	for i := 0; i < l; {
		current := pattern[i]
		if t.v == 0x0 {
			t.v = current
			t.lt = &node{}
			t.eq = &node{}
			t.gt = &node{}
		}

		switch {
		/*
			case current == ':' && pattern[i-1] == '/' && i != lastChar:
				var varEnd int
				var err error
				if varEnd, err = findVarEnd(i+1, pattern); err != nil {
					return err
				}

				t.v = current
				//		case v == '/' && i != lastChar && pattern[i+1] == ':':
				//			if varEnd, err = routeVarEnd(i, pattern); err != nil {
				//				return err
				//			}
				//			t = t.gt
				//			t.v = ':'
				t.varName = string(pattern[i+1 : varEnd])
				//t.lt = &tree{}
				//t.eq = &tree{}
				//t.gt = &tree{}
				//
				i = varEnd
				/*fmt.Println("VAR HANDLE", t)
				if varEnd > lastChar {
					if t.handlers == nil {
						t.handlers = make(mHandlers, 0)
					}
					t.handlers[method] = handler
					fmt.Println("HANDLE LAST VAR", method, pattern, handler, t.handlers)
					return nil
				}
				//
				t = t.lt
		*/

		case current < t.v:
			fmt.Println("LESS THAN", current, string(current), t.v, string(t.v))
			t = t.lt

		case current > t.v:
			fmt.Println("GREATER THAN", string(current), string(t.v))
			t = t.gt

		case i == lastChar:
			fmt.Println("HANDLE END PATH", t, method, pattern, handler)
			if t.handlers == nil {
				t.handlers = make(mHandlers, 0)
			}
			t.handlers[method] = handler
			return nil

		default:
			fmt.Println("EQUAL", string(current), string(t.v))
			t = t.eq
			i++
		}
	}

	return fmt.Errorf("unable to insert handler for key '%s'", pattern)
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

func findVarEnd(start int, path string) (end int, err error) {
	end = start
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
		http.NotFound(w, r)
		return
	}

	/*switch err {
	case ErrMethodNotAllowed:
		notAllowed()
		ctxPool.Put(ctx)
		return
	case ErrNoRoute:
		http.NotFound(w, r)
		ctxPool.Put(ctx)
		return
	default:
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}*/

	handler.ServeHTTP(w, r)
	ctxPool.Put(ctx)
}

var ErrMethodNotAllowed = errors.New("Method verb for this routing pattern is not registered.")
var ErrNoRoute = errors.New("This route cannot be matched.")

// match the route
func (t *node) match(method method, path string, ctx *Context) (http.Handler, error) {
	//fmt.Println("MATCH", method, path)
	var matchEnd int
	var matchedVal string
	var matchedHandler http.Handler
	var ok bool

	for i := 0; i < len(path); {
		v := path[i]
		fmt.Println(t)
		switch {
		case t.v == 0x0:
			return nil, ErrNoRoute

		case v == '/' && t.eq.v == '*':
			if matchedHandler, ok = t.eq.handlers[method]; ok {
				return matchedHandler, nil
			}
			return nil, errors.Wrap(
				ErrMethodNotAllowed,
				fmt.Sprintf("handler not found for wildcard with method '%s'", t.gt.varName, method),
			)

		case v == '/' && t.gt.v == ':':
			fmt.Println("MATCH")
			for matchEnd = i + 1; matchEnd < len(path) && path[matchEnd] != '/'; matchEnd++ {
				if matchEnd == len(path)-1 {
					matchedVal = path[i+1 : matchEnd]

				} else {
					matchedVal = path[i+1 : matchEnd+1]
				}

			}
			fmt.Println("MATCH", matchEnd, i, path, t.gt)
			ctx.Params.Add(t.gt.varName, matchedVal)

			if matchEnd >= len(path)-1 {
				if matchedHandler, ok = t.gt.handlers[method]; ok {
					return matchedHandler, nil
				}
				return nil, errors.Wrap(
					ErrMethodNotAllowed,
					fmt.Sprintf("handler not found for '%s' with method '%s'", t.gt.varName, method),
				)
			}

			i = matchEnd
			t = t.gt.lt

		case v > t.v:
			t = t.gt

		case v < t.v:
			t = t.lt

		case i == len(path)-1:
			if matchedHandler, ok = t.handlers[method]; ok {
				return matchedHandler, nil
			}
			return nil, errors.Wrap(
				ErrMethodNotAllowed,
				fmt.Sprintf("handler not at end of routing pattern with method '%s'", method),
			)

		default:
			t = t.eq
			i++
		}
	}

	return nil, ErrNoRoute
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

func (t *node) longestPrefix(mthd method, key string, ctx *Context) (http.Handler, error) {
	if len(key) < 1 {
		return nil, ErrNoRoute
	}

	length := prefix(t, key, 0)

	return t.match(mthd, key[0:length], ctx)
}

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
