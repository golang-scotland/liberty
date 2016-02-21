package liberty

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/gnanderson/trie"
	"github.com/spf13/viper"
)

const (
	configFile     = "router"
	configLocation = "/etc/router"
)

var (
	runtimeVer string = runtime.Version()
	hostname   string
	openAPI    *trie.Trie
)

// this is used for the metrics and prometheus stuff, it wouldnt break web/api
// handlers but it would cause a lot of headaches if it was removed.
func init() {
	if host, err := os.Hostname(); err == nil {
		hostname = host
	} else {
		panic("no hostname")
	}
}

var conf *Config = loadConfig()

func loadConfig() *Config {
	cfg := &Config{}
	v := viper.New()
	v.SetConfigName(configFile)
	v.AddConfigPath(configLocation)
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("Fatal error reading libertry config: %s\n", err)
	}
	v.Unmarshal(cfg)
	return cfg
}

// Config is the top level configuration for this package, at this moment the
// persisted paramaters are expected to be read from a yaml file.
type Config struct {
	Env           string          `yaml:"env"`
	Profiling     bool            `yaml:"profiling"`
	ProfStatsFile string          `yaml:"profStatsFile"`
	Certs         []*Crt          `yaml:"certs"`
	Proxies       []*Proxy        `yamls:"proxies"`
	Whitelist     []*ApiWhitelist `yaml:"whitelist"`
}

// ApiWhitelist instructs an HTTP API handler to make this open to any remote
// IP. This can be modified to being a 'greylist' entry by specificing a list
// of IP's in CIDR format which the resource is accessible to.
//
// A whitelist IP entry for a path will not override any previous IP restrictions
// in the handler chain. If the root entry in the proxy list has a list of IP
// restrictions, and this whitelist has a list of IP entries, then only the IPs
// in this list which also have a previous entry in the handler chain will be
// allowed access.
type ApiWhitelist struct {
	Path      string   `yaml:"path"`
	IPs       []string `yaml:"ips"`
	Hostnames []string `yaml:"hostnames"`
}

// Crt defines a domain, certificate and keyfile in PEM format
type Crt struct {
	Domain   string
	CertFile string
	KeyFile  string
}

func setTLSConfig(s *http.Server) {
	addr := s.Addr
	if addr == "" {
		addr = ":https"
	}
	// min version doesn't include SSL v3.0, but we don't want that anyway
	// because of the POODLE attack...
	config := &tls.Config{MinVersion: tls.VersionTLS10}

	// *we* will choose the preffered cipher where possible
	config.PreferServerCipherSuites = true
	if s.TLSConfig != nil {
		*config = *s.TLSConfig
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, len(conf.Certs))
	for i := 0; i < len(conf.Certs); i++ {
		config.Certificates[i], err = tls.LoadX509KeyPair(
			conf.Certs[i].CertFile, conf.Certs[i].KeyFile,
		)
		if err != nil {
			panic(err)
		}
	}
	// this will invoke SNI extensions. Note that some (typically older) clients
	// don't support this.
	config.BuildNameToCertificate()

	s.TLSConfig = config
}
