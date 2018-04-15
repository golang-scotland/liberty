package liberty

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.scot/liberty/middleware"
)

// Config is a top level config struct
type Config struct {
	Certs     []*Crt                     `yaml:"certs"`
	Proxies   []*ReverseProxy            `yaml:"proxies"`
	Whitelist []*middleware.ApiWhitelist `yaml:"whitelist"`
}

// Proxy is a reverse HTTP proxy
type Proxy struct {
	config   *Config
	group    *ServerGroup
	secure   map[string]*VHost
	insecure map[string]*VHost
}

// NewProxy returns a Proxy configured for use
func NewProxy(config *Config) *Proxy {
	p := &Proxy{
		config:   config,
		secure:   map[string]*VHost{},
		insecure: map[string]*VHost{},
	}

	servers := make([]*http.Server, 0)

	for _, proxy := range p.config.Proxies {
		host, _ := proxy.hostAndPath()

		if _, ok := p.secure[host]; !ok {
			router := NewRouter()

			p.secure[host] = &VHost{
				host:    host,
				handler: router,
			}

			if len(proxy.HostAlias) > 0 {
				for _, alias := range proxy.HostAlias {
					p.secure[alias] = &VHost{
						host:    alias,
						handler: router,
					}
				}
			}
		}

		if _, ok := p.insecure[host]; !ok {
			p.insecure[host] = &VHost{
				host:    host,
				handler: http.HandlerFunc(middleware.RedirectPerm),
			}
		}

		err := proxy.Configure(p.config.Whitelist, p.secure[host].handler)
		if err != nil {
			fmt.Printf("the proxy for '%s' was not configured - %s\n", proxy.HostPath, err)
			continue
		}

		servers = append(servers, proxy.Servers...)
	}

	p.group = NewServerGroup(p, servers)

	return p
}

func (p *Proxy) vhostDomains() []string {
	domains := make([]string, 0)
	for host := range p.secure {
		domains = append(domains, host)
	}

	return domains
}

const gracePriod = 5 // seconds

// Serve incoming requests between a set of configured reverse proxies, uses
// kernel SO_REUSEPORT which conveniently maps incoming connections to the least
// used socket
func (p *Proxy) Serve() {
	startServer := func(s *server) {
		fmt.Println("server lisening: ", s.s.Addr)
		domains := make([]string, 0)
		domains = append(domains, p.vhostDomains()...)
		fmt.Println("server domains: ", domains)
		log.Println(s.s.Serve(s.Listener(domains)))
	}

	var wg sync.WaitGroup
	wg.Add(len(p.group.servers))

	for _, s := range p.group.servers {
		go startServer(s)
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	log.Println("Draining server connections...")
	for _, s := range p.group.servers {
		go func(srv *http.Server) {
			ctx, cancel := context.WithTimeout(context.Background(), gracePriod*time.Second)
			defer cancel()
			srv.Shutdown(ctx)
			wg.Done()
		}(s.s)
	}

	wg.Wait()
	log.Println("Done, exiting")
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if vhost, ok := p.secure[r.Host]; ok {
		fmt.Println(vhost, r.URL.Path)
		vhost.handler.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}
