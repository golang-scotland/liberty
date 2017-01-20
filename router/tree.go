package router

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

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
	return fmt.Sprintf("[value: %s, t.varName: %s, handlers: %#t]", string(t.v), t.varName, t.handlers)
}

// register a given handler for a pattern with a specific HTTP verb
func (t *tree) handle(method method, pattern string, handler http.Handler) error {
	if handler == nil {
		panic("Handler may not be nil")
	}
	l := len(pattern)
	lastChar := l - 1

	var err error
	var varEnd int

	for i := 0; i < l; {
		v := pattern[i]
		if t.v == 0x0 {
			t.v = v
			t.lt = &tree{}
			t.eq = &tree{}
			t.gt = &tree{}
		}

		switch {
		case v == '/' && i != lastChar && pattern[i+1] == ':':
			if varEnd, err = routeVarEnd(i, pattern); err != nil {
				return err
			}
			t = t.gt
			t.v = ':'
			t.varName = string(pattern[i+2 : varEnd])
			t.lt = &tree{}
			t.eq = &tree{}
			t.gt = &tree{}

			i = varEnd
			if varEnd > lastChar {
				if t.handlers == nil {
					t.handlers = make(mHandlers, 0)
				}
				t.handlers[method] = handler
				return nil
			}

			t = t.lt

		case v < t.v:
			t = t.lt

		case v > t.v:
			t = t.gt

		case i == lastChar:
			if t.handlers == nil {
				t.handlers = make(mHandlers, 0)
			}
			t.handlers[method] = handler
			return nil

		default:
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
	var handler http.Handler
	var err error
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

	handler, err = t.match(method, r.URL.Path, ctx)

	switch err {
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
	}

	handler.ServeHTTP(w, r)
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
			return nil, errors.Wrap(
				ErrMethodNotAllowed,
				fmt.Sprintf("handler not found for wildcard with method '%s'", t.gt.varName, method),
			)

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

func (t *tree) longestPrefix(mthd method, key string, ctx *Context) (http.Handler, error) {
	if len(key) < 1 {
		return nil, ErrNoRoute
	}

	length := prefix(t, key, 0)

	return t.match(mthd, key[0:length], ctx)
}

func prefix(t *tree, key string, index int) int {
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
