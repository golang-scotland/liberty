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
			p.secure[host] = &VHost{
				host:    host,
				handler: NewRouter(),
			}
		}
		if _, ok := p.insecure[host]; !ok {
			p.insecure[host] = &VHost{
				host:    host,
				handler: http.HandlerFunc(middleware.RedirectPerm),
			}
		}

		err := proxy.Configure(p.config.Whitelist, p.secure[host].handler, p.serveInsecure)
		if err != nil {
			fmt.Printf("the proxy for '%s' was not configured - %s\n", proxy.HostPath, err)
			continue
		}

		servers = append(servers, proxy.Servers...)
	}

	p.group = NewServerGroup(p, servers)
	p.group.setTLSConfig(p.vhostDomains())

	return p
}

func (b *Proxy) vhostDomains() []string {
	domains := make([]string, 0)
	for host, _ := range b.secure {
		domains = append(domains, host)
	}

	return domains
}

const gracePriod = 5 // seconds

// Serve incoming requests between a set of configured reverse proxies, uses
// kernel SO_REUSEPORT which conveniently maps incoming connections to the least
// used socket
func (b *Proxy) Serve() {
	sig := make(chan os.Signal)

	startServer := func(s *server, wg *sync.WaitGroup) {
		fmt.Println("server lisening: ", s.s.Addr)
		log.Println(s.s.Serve(s.Listener()))
	}

	var wg sync.WaitGroup
	wg.Add(len(b.group.servers))

	for _, s := range b.group.servers {
		go startServer(s, &wg)
	}

	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
	log.Println("Draining server connections...")
	for _, s := range b.group.servers {
		go func() {
			ctx, _ := context.WithTimeout(context.Background(), gracePriod*time.Second)
			s.s.Shutdown(ctx)
			wg.Done()
		}()
	}

	wg.Wait()
	log.Println("Done, exiting")
}

func (b *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if vhost, ok := b.secure[r.Host]; ok {
		fmt.Println(vhost, r.URL.Path)
		vhost.handler.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

func (b *Proxy) serveInsecure(w http.ResponseWriter, r *http.Request) {
	if s, ok := b.insecure[r.Host]; ok {
		s.handler.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}
