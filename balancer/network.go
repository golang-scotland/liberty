package balancer

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coreos/go-systemd/activation"
)

const (
	SO_REUSEPORT = 0x0F
)

// serveSocketListener takes a pair of http servers and adds systemd based
// socket activated listeners for ports 80 and 443
func serveSocketListener(srv, secureSrv *http.Server) error {
	listeners, err := activation.Listeners(true)
	if err != nil {
		panic(err)
	}

	if len(listeners) != 2 {
		panic(fmt.Sprintf("Unexpected number of socket activation fds: %d", len(listeners)))
	}

	// "Serve" never returns unless there's a serious problem so launch it in
	// a goroutine.
	ln := listeners[0]
	go func() {
		fmt.Println(srv.Serve(ln))
	}()

	lnSecure := listeners[1]
	return serveSecureSocketListener(secureSrv, lnSecure)
}

// load the certificate and key pair in pem format and create a new TLS listener
// from a net.Listener
func serveSecureSocketListener(srv *http.Server, ln net.Listener) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	if srv.TLSConfig != nil {
		*config = *srv.TLSConfig
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
			return err
		}
	}

	tlsListener := tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, config)
	return srv.Serve(tlsListener)
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
