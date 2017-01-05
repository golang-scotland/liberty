package main

import (
	"fmt"
	"log"
	"net/http"

	_ "net/http/pprof"

	"golang.scot/liberty/router"
)

func httpHandlerFunc(w http.ResponseWriter, r *http.Request) {}

func main() {
	r := router.NewHTTPRouter()
	r.Use(nil)
	r.Handle("/", http.HandlerFunc(httpHandlerFunc))

	go func() {
		fmt.Println(http.ListenAndServe(":7777", r))
	}()
	fmt.Println("MEH")
	log.Println(http.ListenAndServe("localhost:6060", nil))
}
