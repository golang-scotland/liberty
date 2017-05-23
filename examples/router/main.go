package main

import (
	"log"
	"net/http"

	_ "net/http/pprof"

	"golang.scot/liberty/router"
)

func httpHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello World!"))
}

func main() {
	r := router.NewRouter()
	r.Get("/", http.HandlerFunc(httpHandlerFunc))

	go func() {
		log.Fatal(http.ListenAndServe(":8181", r))
	}()
	log.Fatal(http.ListenAndServe(":8282", nil))
}
