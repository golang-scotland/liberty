package router

import "sync"

type Context struct {
	Params Params
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

func (c *Context) Reset() {
	c.Params = c.Params[:0]
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
