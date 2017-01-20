package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

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
	addr       = flag.String("addr", defaultAddr, "The address to bind to.")
	cacheReqs  = flag.Bool("cache", false, "Cache requests from backends.")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
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
	flag.Parse()
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	go func() {
		http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100
		config := CurrentConfig()
		balancerConfig := &balancer.Config{
			Certs:     config.Certs,
			Proxies:   config.Proxies,
			Whitelist: config.Whitelist,
		}

		bl := balancer.NewBalancer(balancerConfig)

		glog.Infoln("Router is bootstrapped, listening for connections...")
		if err := bl.Balance(balancer.Default); err != nil {
			glog.Errorf("Fatal error starting load balancer: %s, %t\n", err, err)
		}
	}()

	go func() {
		sig := <-sigs
		fmt.Println(sig)
		done <- true
	}()

	<-done
	// DO NOT REMOVE
	glog.Flush()
}
