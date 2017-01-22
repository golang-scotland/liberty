package router

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	"GET":    GET,
	"POST":   POST,
	"PUT":    PUT,
	"PATCH":  PATCH,
	"HEAD":   HEAD,
	"RANGE":  RANGE,
	"DELETE": DELETE,
}

type mHandlers map[method]http.Handler

// a ternary search trie (tree for the avoidance of pronunciation doubt)
type tree struct {
	lt       *tree
	eq       *tree
	gt       *tree
	v        byte
	handlers mHandlers
	varName  string
}

func (t *tree) String() string {
	return fmt.Sprintf("[value: %s, t.varName: %s]", string(t.v), t.varName)
}

func (t *tree) handle(method method, path string, handler http.Handler) error {
	if handler == nil {
		panic("nil group")
	}
	l := len(path)
	lastChar := l - 1

	var err error
	var varEnd int

	for i := 0; i < l; {
		v := path[i]
		if t.v == 0x0 {
			t.v = v
			t.lt = &tree{}
			t.eq = &tree{}
			t.gt = &tree{}
		}

		switch {
		case v == '/' && i != lastChar && path[i+1] == ':':
			if varEnd, err = routeVarEnd(i, path); err != nil {
				return err
			}

			t = t.gt
			t.v = ':'
			t.varName = string(path[i+2 : varEnd])
			t.lt = &tree{}
			t.eq = &tree{}
			t.gt = &tree{}

			i = varEnd
			t.addMethodHandler(method, handler)

			if varEnd > lastChar {
				return nil
			}

			t = t.lt

		case v < t.v:
			t = t.lt

		case v > t.v:
			t = t.gt

		case i == lastChar:
			t.addMethodHandler(method, handler)
			return nil

		default:
			t = t.eq
			i++
		}
	}

	return fmt.Errorf("unable to insert handler for key '%s'", path)
}

func (t *tree) addMethodHandler(m method, h http.Handler) {
	if t.handlers == nil {
		t.handlers = make(mHandlers, 0)
	}

	t.handlers[m] = h
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

func (t *tree) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := ctxPool.Get().(*Context)
	//ctx := r.Context().Value(CtxKey).(*Context)

	h, err := t.match(methods[r.Method], r.URL.Path, ctx)
	if h == nil {
		if strings.HasSuffix(r.URL.Path, "*") {
			if h, err = t.matchWildcard(methods[r.Method], r.URL.Path[:len(r.URL.Path)-1], ctx); h != nil {
				h.ServeHTTP(w, r)
				return
			}
		}

		switch err {
		case ErrMethodNotAllowed:
			http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		case ErrNoRoute:
			http.NotFound(w, r)
		}

		return
	}

	h.ServeHTTP(w, r)
	ctxPool.Put(ctx)
}

var ErrMethodNotAllowed = errors.New("Method verb for this routing pattern is not registered.")
var ErrNoRoute = errors.New("This route cannot be matched.")

// match the route
func (t *tree) match(method method, path string, ctx *Context) (http.Handler, error) {
	var matchEnd int
	var matchedVal string
	var matchedHandler http.Handler
	var ok bool

	for i := 0; i < len(path); {
		v := path[i]

		switch {
		case t.v == 0x0:
			return nil, ErrNoRoute

		case v == '/' && t.eq.v == '*':
			if matchedHandler, ok = t.eq.handlers[method]; ok {
				return matchedHandler, nil
			}
			return nil, ErrMethodNotAllowed

		case v == '/' && t.gt.v == ':':
			for matchEnd = i + 1; matchEnd < len(path) && path[matchEnd] != '/'; matchEnd++ {
				if matchEnd == len(path)-1 {
					matchedVal = path[i+1 : matchEnd]

				} else {
					matchedVal = path[i+1 : matchEnd+1]
				}

			}
			ctx.Params.Add(t.gt.varName, matchedVal)

			if matchEnd >= len(path)-1 {
				if matchedHandler, ok = t.gt.handlers[method]; ok {
					return matchedHandler, nil
				}
				return nil, ErrMethodNotAllowed
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
			return nil, ErrMethodNotAllowed

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

func (t *tree) matchWildcard(m method, key string, ctx *Context) (http.Handler, error) {
	prefixEnd := t.prefixLength(t, key, 0)

	return t.match(m, key[0:prefixEnd], ctx)
}

func (t *tree) prefixLength(search *tree, key string, index int) int {
	if index == len(key) {
		return 0
	}

	length := 0
	recLen := 0
	v := key[index]

	if v < t.v {
		recLen = t.prefixLength(t.lt, key, index)
	} else if v > t.v {
		recLen = t.prefixLength(t.gt, key, index)
	} else {
		if t.v != 0x0 {
			length = index + 1
		}
		recLen = t.prefixLength(t.eq, key, index+1)
	}
	if length > recLen {
		return length
	}

	return recLen
}
