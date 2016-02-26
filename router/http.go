// Package router implements a ternary search tree based HTTP router. The main
// focus of this package is to support using a single HTTP request muxer that
// multiple HTTP servers.
package router

import (
	"fmt"
	"net/http"
)

type HttpRouter struct {
	tree *tree
}

func (h *HttpRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostPath := r.Host + r.URL.Path
	if handler := h.Get(hostPath); handler != nil {
		handler.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *HttpRouter) Put(key string, serverGroup *ServerGroup) error {
	return h.put(key, serverGroup)
}

func (h *HttpRouter) Get(key string) http.Handler {
	var sg *ServerGroup
	if sg = h.get(key); sg == nil {
		if sg = h.longestPrefix(key); sg == nil {
			return nil
		}
	}
	return sg.leastUsed()
}

func (h *HttpRouter) Getc(key string) *ServerGroup {
	return h.getc(key)
}

type tree struct {
	lt *tree
	eq *tree
	gt *tree
	v  byte
	sg *ServerGroup
}

func (h *HttpRouter) get(key string) *ServerGroup {
	l := len(key)
	n := h.tree
	for i := 0; i < l; {
		v := key[i]
		if n.v == 0x0 {
			return nil
		} else if v < n.v {
			n = n.lt
		} else if v > n.v {
			n = n.gt
		} else if (i == len(key)-1) && (n.sg != nil) {
			return n.sg
		} else {
			n = n.eq
			i++
		}
	}
	return nil
}

func (h *HttpRouter) longestPrefix(key string) *ServerGroup {
	if len(key) < 1 {
		return nil
	}

	length := h.prefix(h.tree, key, 0)
	return h.get(key[0:length])
}

func (h *HttpRouter) prefix(n *tree, key string, index int) int {
	if index == len(key) || n == nil {
		return 0
	}

	length := 0
	recLen := 0
	v := key[index]

	if v < n.v {
		recLen = h.prefix(n.lt, key, index)
	} else if v > n.v {
		recLen = h.prefix(n.gt, key, index)
	} else {
		if n.v != 0x0 {
			length = index + 1
		}
		recLen = h.prefix(n.eq, key, index+1)
	}
	if length > recLen {
		return length
	}
	return recLen
}

func (h *HttpRouter) getc(key string) *ServerGroup {
	l := len(key)
	n := h.tree
	for i := 0; i < l; {
		v := key[i]
		switch {

		case n.v == 0x0:
			return nil
		case v < n.v:
			n = n.lt
		case v > n.v:
			n = n.gt
		case i == len(key)-1 && n.sg != nil:
			return n.sg
		default:
			n = n.eq
			i++
		}

	}
	return nil
}
func (h *HttpRouter) put(key string, group *ServerGroup) error {
	if group == nil {
		panic("nil group")
	}
	b := []byte(key)
	n := h.tree
	if n == nil {
		n = &tree{}
		h.tree = n
	}
	for i := 0; i < len(b); {
		v := b[i]
		if n.v == 0x0 {
			n.v = v
			n.lt = &tree{}
			n.eq = &tree{}
			n.gt = &tree{}
		}

		if v < n.v {
			n = n.lt
		} else if v > n.v {
			n = n.gt
		} else if i == (len(b) - 1) {
			n.sg = group
			return nil
		} else {
			n = n.eq
			i++
		}
	}
	return fmt.Errorf("unable to insert handler for key '%s'", key)
}
