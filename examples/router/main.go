package main

import (
	"fmt"
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
	r.Use(nil)
	r.Get("/", http.HandlerFunc(httpHandlerFunc))

	go func() {
		fmt.Println(http.ListenAndServe(":7777", r))
	}()
	log.Println(http.ListenAndServe("localhost:6060", nil))
}
