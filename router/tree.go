package router

import (
	"errors"
	"fmt"
	"net/http"
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
)

var methods = map[string]method{
	"GET":     GET,
	"POST":    POST,
	"PUT":     PUT,
	"PATCH":   PATCH,
	"OPTIONS": OPTIONS,
	"HEAD":    HEAD,
	"RANGE":   RANGE,
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
			if varEnd > lastChar {
				t.handlers[method] = handler
				return nil
			}

			t = t.lt

		case v < t.v:
			t = t.lt

		case v > t.v:
			t = t.gt

		case i == lastChar:
			t.handlers[method] = handler
			return nil

		default:
			t = t.eq
			i++
		}
	}

	return fmt.Errorf("unable to insert handler for key '%s'", path)
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

	h := t.match(r.URL.Path, ctx)
	if h == nil {
		http.NotFound(w, r)
		return
	}

	h.ServeHTTP(w, r)
	ctxPool.Put(ctx)
}

var ErrMethodNotAllowed = errors.New("Method verb for this routing pattern is not registered.")
var ErrNoRoute = errors.New("This route cannot be amtched.")

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
				if matchedHandler, ok = t.eq.handlers[method]; ok {
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
			if matchedHandler, ok = t.eq.handlers[method]; ok {
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
