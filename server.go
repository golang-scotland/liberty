package liberty

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"golang.scot/liberty/env"
)

const letsEncryptSandboxUrl = "https://acme-staging.api.letsencrypt.org/directory"

// Crt defines a domain, certificate and keyfile
type Crt struct {
	Domain   string
	CertFile string
	KeyFile  string
}

type server struct {
	open    uint32
	s       *http.Server
	handler http.Handler
}

func (s *server) openConns() uint32 {
	return atomic.LoadUint32(&s.open)
}

func (s *server) trackState(c net.Conn, cs http.ConnState) {
	switch cs {
	case http.StateNew:
		// Do we care more about StateNew than StateActive?
		atomic.AddUint32(&s.open, 1)
	case http.StateClosed, http.StateHijacked:
		// new/active connections will eventually transition to hijacked or closed and
		// with both being terminal states we will decrement the count on a callback
		// from either of these states.
		//atomic.AddUint32(&s.open, ^uint32(0))
	}
}

// ServerGroup defines a grouping of servers which can be have requests routed
// to via a liberty router
type ServerGroup struct {
	w       *sync.Mutex
	servers []*server
}

// NewServerGroup creates a server group from a balancer and a slice of standard
// library http servers
func NewServerGroup(balancer *Balancer, servers []*http.Server) *ServerGroup {
	sg := &ServerGroup{
		w:       &sync.Mutex{},
		servers: []*server{},
	}

	for _, s := range servers {
		if !strings.HasSuffix(s.Addr, ":80") {
			s.Handler = balancer
		}
		srv := &server{
			s:       s,
			handler: s.Handler,
		}

		s.ConnState = srv.trackState
		sg.servers = append(sg.servers, srv)
	}

	return sg
}

func (sg *ServerGroup) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	sg.leastUsed().ServeHTTP(resp, req)
}

// HTTPServers returns the standard library http servers associated with this
// ServerGroup
func (sg *ServerGroup) HTTPServers() []*http.Server {
	servers := make([]*http.Server, len(sg.servers), len(sg.servers))
	for i, s := range sg.servers {
		servers[i] = s.s
	}
	return servers
}

func (sg *ServerGroup) leastUsed() http.Handler {
	var h http.Handler
	if sg.w == nil {
		sg.w = &sync.Mutex{}
	}
	sg.w.Lock()
	sort.Sort(sg)
	h = sg.servers[0].handler
	sg.w.Unlock()
	return h
}

func (sg *ServerGroup) Len() int {
	return len(sg.servers)
}

func (sg *ServerGroup) Less(i, j int) bool {
	return sg.servers[i].openConns() < sg.servers[j].openConns()
}

func (sg *ServerGroup) Swap(i, j int) {
	sg.servers[i], sg.servers[j] = sg.servers[j], sg.servers[i]
}

func setTLSConfig(s *http.Server, domains []string) {
	if strings.HasSuffix(s.Addr, ":80") {
		return
	}

	addr := s.Addr
	if addr == "" {
		addr = ":https"
	}
	// min version doesn't include SSL v3.0, but we don't want that anyway
	// because of the POODLE attack...
	//config := &tls.Config{MinVersion: tls.VersionTLS10}

	// *we* will choose the preffered cipher where possible
	//config.PreferServerCipherSuites = true
	//if s.TLSConfig != nil {
	//		*config = *s.TLSConfig
	//}
	//if config.NextProtos == nil {
	//	config.NextProtos = []string{"http/1.1"}
	//}

	//fmt.Println("DOMAINS", domains)

	// Lets Encrypt!
	s.TLSConfig = &tls.Config{}
	m := &autocert.Manager{
		Client:     newAcmeClient(),
		Cache:      autocert.DirCache(os.Getenv("ACME_CACHE")),
		Email:      os.Getenv("ACME_EMAIL"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
	}
	s.TLSConfig.GetCertificate = m.GetCertificate

	// this will invoke SNI extensions. Note that some (typically older) clients
	// don't support this.
	// config.BuildNameToCertificate()

	//s.TLSConfig = config
}

func newAcmeClient() *acme.Client {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	client := &acme.Client{Key: key}

	if env.Get() != env.Prod {
		client.DirectoryURL = letsEncryptSandboxUrl
	}

	return client
}

type VHost struct {
	host    string
	handler http.Handler
}

func (v *VHost) String() string {
	return v.host
}

func (v *VHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host != v.host {
		panic(fmt.Sprintf("vhost '%s' cannot serve traffic for '%s'\n", v.host, r.Host))
	}

	v.handler.ServeHTTP(w, r)
}
