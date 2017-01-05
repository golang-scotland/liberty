package middleware

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"text/template"

	"github.com/NYTimes/gziphandler"
	"github.com/gnanderson/trie"
	"github.com/prometheus/client_golang/prometheus"
	"golang.scot/liberty/env"
)

const (
	apiHandler      = "api"
	webHandler      = "web"
	promHandler     = "prometheus"
	redirectHandler = "redirect"
)

type HelloWorld struct{}

func (hw *HelloWorld) Chain(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//w.Write([]byte("Hello World!"))
	})
}

type GoGet struct {
	Host string
	Path string
}

func GoGetHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			if r.Form.Get("go-get") == "1" {
				gg := &GoGet{
					Host: r.Host,
					Path: r.URL.Path,
				}
				if err := ggTpl.Execute(w, gg); err != nil {
					http.Error(w, err.Error(), 500)
				}
				return
			}
			h.ServeHTTP(w, r)
		}
	})
}

var ggTpl *template.Template = getGGTpl()

func getGGTpl() *template.Template {
	goGetTpl := `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<meta name="go-import" content="{{ .Host }}{{ .Path }} git https://github.com/golang-scot/liberty }}">
<meta http-equiv="refresh" content="0; url=https://godoc.org/{{ .Host }}{{ .Path }}">
</head>
<body>
<a href="https://godoc.org/{{ .Host }}{{ .Path }}">{{ .Host }}{{ .Path }}</a>.
</body>
</html>
`
	tpl, err := template.New("GGTPL").Parse(goGetTpl)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return tpl
}

type GzipHandler struct{}

func (hl *GzipHandler) Chain(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gziphandler.GzipHandler(h)
	})
}

// IPRestrictedHandler does what it says on the tin - names not down, not getting
// in!
type IPRestrictedHandler struct {
	Allowed     []*net.IPNet
	handlerType string
	openPaths   *trie.Trie
}

func (rh *IPRestrictedHandler) Chain(h http.Handler) http.Handler {
	appEnv := env.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := parseForwarderIP(r, appEnv)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// don't block any dev environment
		/*
			if appEnv == env.Dev {
				h.ServeHTTP(w, r)
				return
			}
		*/

		// if this is an API handler we need to restrict access by default
		// unless the request path has a prefix in the trie of open paths. The
		// order of access should be...
		//
		// 1. Open paths are allowed
		// 2. If the path is not open but the IP is allowed, proceed
		if rh.handlerType == apiHandler && rh.openPaths.LongestPrefix(r.URL.String()) != "" {
			h.ServeHTTP(w, r)
			return
		}

		for _, ipNet := range rh.Allowed {
			if ipNet.Contains(ip) {
				h.ServeHTTP(w, r)
				return
			}
		}

		// At this point it could be a paypal IPN notification, so we'll do one
		// final check before returning a 403. Until we have better test coverage
		// please leave this paypal specific chunk of code in place. This final
		// check does a reverse lookup on the incoming IP address and if it matches
		// paypal.com then we serve the request.
		if !(appEnv == env.Dev) {
			if ValidIPNSource(ip) {
				h.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, fmt.Sprintf("☹ - IP %s is not allowed...", ip), 403)
	})
}

// try to retrive the forwarding IP
func parseForwarderIP(r *http.Request, appEnv env.Env) (ip net.IP, err error) {
	var remote string
	// Outside of production env we always use the remote address
	if appEnv != env.Prod {
		remote, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return nil, err
		}
	} else {
		remote = r.Header.Get("X-Forwarded-For")
		if remote == "" {
			remote, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return nil, err
			}
		}
		// some forwarded for headers contain a sequence of IP's, in this case
		// we are interested in the first one.
		if strings.Contains(remote, ",") {
			remote = strings.Split(remote, ",")[0]
		}
	}

	return net.ParseIP(remote), nil
}

// ValidIPNSource checks whether the request originates at paypal
func ValidIPNSource(ip net.IP) bool {
	if names, err := net.LookupAddr(ip.String()); err == nil {
		for _, name := range names {
			if strings.HasSuffix(name, "paypal.com.") {
				return true
			}
		}
	}
	return false
}

// InstrumentedHandler often starts the handler chain by initialising some
// prometheus based monitoring and metrics.
type InstrumentedHandler struct {
	Name string
}

func (ih *InstrumentedHandler) Chain(h http.Handler) http.Handler {
	return http.HandlerFunc(prometheus.InstrumentHandler(ih.Name, h))
}

func redir(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("https://%s%s", r.Host, r.RequestURI)
	http.Redirect(w, r, url, 302)
}

// hijack the connection and dial the backend to do the HTTP upgrade dance
func websocketProxy(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, "Error contacting backend server.", 500)
			log.Printf("Error dialing websocket backend %s: %v", target, err)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Not a hijacker?", 500)
			return
		}
		nc, _, err := hj.Hijack()
		if err != nil {
			log.Printf("Hijack error: %v", err)
			return
		}
		defer nc.Close()
		defer d.Close()

		err = r.Write(d)
		if err != nil {
			log.Printf("Error copying request to target: %v", err)
			return
		}

		errc := make(chan error, 2)
		cp := func(dst io.Writer, src io.Reader) {
			_, err := io.Copy(dst, src)
			errc <- err
		}
		go cp(d, nc)
		go cp(nc, d)
		<-errc
	})
}
