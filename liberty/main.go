package main

import (
	"flag"
	"fmt"
	"os"

	"net/http"
	_ "net/http/pprof"

	"github.com/golang/glog"
	"golang.scot/liberty/balancer"
)

const (
	defaultAddr = ":80"
	svcName     = "router"
)

const (
	modeDevelopment = iota
	modeServing
)

var (
	addr      = flag.String("addr", defaultAddr, "The address to bind to.")
	cacheReqs = flag.Bool("cache", false, "Cache requests from backends.")
)

// print the usage help text for --help / -h
func usage() {
	fmt.Fprintf(os.Stderr,
		"\nUsage:\thttp-router [options]\n\n"+
			"e.g.\thttp-router -addr="+defaultAddr+"\n\n"+
			"Options\n"+
			"-------\n\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	//flag.Parse()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100
	bl := balancer.NewBalancer()

	glog.Infoln("Router is bootstrapped, listening for connections...")
	if err := bl.Balance(balancer.BestEffort); err != nil {
		glog.Errorf("Fatal error starting load balancer: %s, %t\n", err, err)
	}

	// DO NOT REMOVE
	glog.Flush()
}
