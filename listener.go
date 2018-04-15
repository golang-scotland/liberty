package liberty

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.scot/liberty/env"

	reuseport "github.com/kavu/go_reuseport"
)

// Listener listens on the standard TLS port (443) on all interfaces
// and returns a net.Listener returning *tls.Conn connections.
//
// The returned Listener also enables TCP keep-alives on the accepted
// connections. The returned *tls.Conn are returned before their TLS
// handshake has completed.
//
// Unlike NewListener, it is the caller's responsibility to initialize
// the Manager m's Prompt, Cache, HostPolicy, and other desired options.
func (s *server) Listener(domains []string) net.Listener {
	// Lets Encrypt!
	m := &autocert.Manager{
		Client:     newAcmeClient(),
		Cache:      autocert.DirCache(os.Getenv("ACME_CACHE")),
		Email:      os.Getenv("ACME_EMAIL"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
	}

	h := m.HTTPHandler(nil)
	if strings.HasSuffix(s.s.Addr, ":80") {
		s.handler = h
		s.s.Handler = h
	}

	config := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2", "http/1.1"},
		GetCertificate:           m.GetCertificate,
	}

	ln := &listener{
		s:      s,
		config: config,
	}

	ln.tcpListener, ln.tcpListenErr = reuseport.NewReusablePortListener("tcp4", s.s.Addr)
	return ln
}

type listener struct {
	s            *server
	config       *tls.Config
	tcpListener  net.Listener
	tcpListenErr error
}

func (ln *listener) Accept() (net.Conn, error) {
	if ln.tcpListenErr != nil {
		return nil, ln.tcpListenErr
	}
	conn, err := ln.tcpListener.Accept()
	if err != nil {
		return nil, err
	}
	tcpConn := conn.(*net.TCPConn)

	// Because Listener is a convenience function, help out with
	// this too.  This is not possible for the caller to set once
	// we return a *tcp.Conn wrapping an inaccessible net.Conn.
	// If callers don't want this, they can do things the manual
	// way and tweak as needed. But this is what net/http does
	// itself, so copy that. If net/http changes, we can change
	// here too.
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(3 * time.Minute)

	var finalConn net.Conn
	finalConn = tcpConn
	if strings.HasSuffix(ln.s.s.Addr, ":443") {
		finalConn = tls.Server(tcpConn, ln.config)
	}
	return finalConn, nil
}

func (ln *listener) Addr() net.Addr {
	if ln.tcpListener != nil {
		return ln.tcpListener.Addr()
	}
	// net.Listen failed. Return something non-nil in case callers
	// call Addr before Accept:
	return &net.TCPAddr{IP: net.IP{0, 0, 0, 0}, Port: 443}
}

func (ln *listener) Close() error {
	if ln.tcpListenErr != nil {
		return ln.tcpListenErr
	}
	return ln.tcpListener.Close()
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
