package liberty

import (
	"context"
	"net/http"
	"sync"
)

type Context struct {
	Params Params
}

func (c *Context) Reset() {
	c.Params = c.Params[:0]
}

func routingContext(ctx context.Context) *Context {
	return ctx.Value(CtxKey).(*Context)
}

func RouteParam(r *http.Request, key string) string {
	if ctx := routingContext(r.Context()); ctx != nil {
		return ctx.Params.Get(key)
	}
	return ""
}

type Params []Param

type Param struct {
	Key   string
	Value string
}

func (ps *Params) Add(key, value string) {
	*ps = append(*ps, Param{key, value})
}

func (ps Params) Get(name string) string {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value
		}
	}

	return ""
}

var ctxPool = sync.Pool{
	New: func() interface{} { return &Context{} },
}

type ctxKey struct {
	name string
}

func (key *ctxKey) String() string {
	return "router context " + key.name
}

var (
	CtxKey = &ctxKey{"LibertyRoute"}
)
