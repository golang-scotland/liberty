package middleware

import "net/http"

// Chainable describes a handler which wraps a handler.
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

// Link the chain, note that we decrement the slice index which means the handler
// passed in the invocation is linked with the LAST handler in the slice.
func (ch Chain) Link(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	for i := len(ch.handlers) - 1; i >= 0; i-- {
		h = ch.handlers[i].Chain(h)
	}

	return h
}
