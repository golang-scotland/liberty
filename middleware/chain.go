package middleware

import "net/http"

// Chainable describes a handler which wraps a handler. By design there is no
// guarantee that a chainable handler will call the next one in the chain. To
// be chainable the object must also be able to serve HTTP requests and thus
// it will also itself satisfy the standard library http.Handler interface
type Chainable interface {
	Chain(h http.Handler) http.Handler
}

// Chain is a series of chainable http handlers
type Chain struct {
	handlers []Chainable
}

// ChainFunc is an adapter to allow chaining of externally defined middlewares
type ChainFunc func(http.Handler) http.Handler

// Chain calls f(h) and returns an http.Handler
func (f ChainFunc) Chain(h http.Handler) http.Handler {
	return f(h)
}

// NewChain initiates the chain
func NewChain(handlers ...Chainable) *Chain {
	ch := &Chain{}
	ch.handlers = append(ch.handlers, handlers...)
	return ch
}

// Link the chain
func (ch Chain) Link(h http.Handler) http.Handler {
	var last http.Handler

	if h == nil {
		last = http.DefaultServeMux
	} else {
		last = h
	}

	for i := len(ch.handlers) - 1; i >= 0; i-- {
		last = ch.handlers[i].Chain(last)
	}

	return last
}
