package balancer

import (
	"hash/fnv"
	"net/http"
	"strconv"

	"golang.scot/liberty/env"
)

var DefaultTypes = []string{
	"text/html; charset=UTF-8",
	"text/html",
	"application/javascript",
	"text/css",
	"text/xml",
	"text/plain",
}

func eTagMatch(req *http.Request, res *http.Response) bool {
	if env.Get() != env.Prod {
		return false
	}
	if if_none_match := req.Header.Get("If-None-Match"); if_none_match != "" {
		if res.StatusCode == 200 && res.Header.Get("Etag") == if_none_match {
			res.StatusCode = 304
			res.Status = "304 Not Modified"
			res.Body.Close()
			res.ContentLength = 0
			return true
		}
	}
	return false
}

func setEtag(res *http.Response) {
	var contentType = res.Header.Get("Content-Type")
	switch contentType {
	case "text/css", "text/css; charset=UTF-8", "text/javascript", "application/javascript":
		hash := fnv.New32a()
		hash.Write([]byte(res.Request.URL.String()))
		sum := hash.Sum32()
		res.Header.Set("Etag", strconv.Itoa(int(sum)))
	}
}

func maxAge(res *http.Response) {
	var contentType = res.Header.Get("Content-Type")
	switch contentType {
	case "text/html", "text/html; charset=UTF-8", "application/json", "text/xml":
		res.Header.Set("Cache-Control", "no-store")
	case "image/png", "image/jpeg", "image/gif":
		res.Header.Set("Cache-Control", "public, max-age=2419200")
	case "text/css", "text/css; charset=UTF-8", "text/javascript", "application/javascript":
		res.Header.Set("Cache-Control", "public, max-age=2419200")
	}
}
