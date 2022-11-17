package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	valid "github.com/asaskevich/govalidator"
	"github.com/rs/xid"
)

var (
	PROXY_NAME = "Dalal Proxy"
)

func init() {
	art := `
Dalal Proxy
------------
Simple proxy implementation & demonstration <->
Usage:
-workers (default: 1)
==> Specify number of workers (workers:requests)
-hosts (default: "")
==> Specify upstream server | server(s) using comma
-scheme (default: https)
==> Protocol scheme for upstream server
-disable-cache (default: false)
==> Remove Cache-Control header from response

Example:
dalal -workers=40 -hosts=service1.com,service2.com -scheme=https -disable-cache=true
`
	fmt.Println(art)
}

var requests map[string]bool = map[string]bool{}
var mu sync.Mutex
var upstreamHosts []string
var rrIndex = 0 // round robin index
var scheme = ""
var client = http.Client{Timeout: time.Second * 10}
var disableCache = false

func main() {
	var channel = make(chan RPC)
	workers := flag.Int("workers", 1, "number of workers")
	rawScheme := flag.String("scheme", "https", "upstream protocol scheme, supported [http, https] (default: http)")
	rawUpstreamHosts := flag.String("hosts", "", "upstream host(s) service1.com, service2.com")
	port := flag.Int("port", 65535, "port number in which proxy is running (default: 65535)")
	rawDisableCache := flag.Bool("disable-cache", false, "remove cache-control header in response (public cache)")
	flag.Parse()

	if *workers <= 0 {
		log.Fatal("error: worker size should be grater than 0")
	}

	filteredUpstreamHosts := strings.Trim(*rawUpstreamHosts, "")
	upstreamHosts = strings.Split(filteredUpstreamHosts, ",")
	if len(upstreamHosts) <= 0 {
		log.Fatal("error: please supply hostname(s), you can supply multiple names by comma separated")
	}

	for _, upstreamHost := range upstreamHosts {
		if !valid.IsDNSName(upstreamHost) {
			log.Fatalf("error: invalid hostname %s", upstreamHost)
		}
	}

	supportedProtoScheme := []string{"http", "https"}
	if !contains(supportedProtoScheme, *rawScheme) {
		log.Fatalf("error: invalid protocol scheme %s (default: https)", *rawScheme)
	}
	scheme = *rawScheme

	if !valid.IsPort(fmt.Sprint(*port)) {
		log.Fatalf("error: invalid port number %d", *port)
	}

	disableCache = *rawDisableCache

	for i := 0; i < *workers; i += 1 {
		go proxy(channel)
	}

	log.Printf("workers: %d", *workers)
	log.Printf("listening on port: %d", *port)

	http.ListenAndServe(fmt.Sprintf(":%d", *port), handler(channel))
}

type RPC struct {
	w       http.ResponseWriter
	r       *http.Request
	reqId   string
	hostIdx int
}

func proxy(ch <-chan RPC) {
	for c := range ch {
		data := c

		reqURL := data.r.URL
		reqURL.Host = upstreamHosts[data.hostIdx]
		reqURL.Scheme = scheme

		upstreamReq := http.Request{
			Method: data.r.Method,
			Form:   data.r.Form,
			URL:    reqURL,
		}

		res, err := client.Do(&upstreamReq)
		if err != nil {
			data.w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(data.w, "error while proxy")
			requests[data.reqId] = true
			return
		}
		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			data.w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(data.w, "error while reading response from proxy")
			requests[data.reqId] = true
			return
		}

		for name, values := range res.Header {
			for _, v := range values {
				data.w.Header().Add(name, v)
			}
		}

		var securityHeaders []string = []string{
			"server",
			"via",
			"X-Powered-By",
		}
		if disableCache {
			securityHeaders = append(securityHeaders, "cache-control")
		}

		for _, securityHeader := range securityHeaders {
			data.w.Header().Del(securityHeader)
		}

		data.w.Header().Add("server", PROXY_NAME)
		log.Printf("%s | %s | %d | %s", data.r.Method, reqURL.Host, res.StatusCode, data.r.URL.Path)
		data.w.Write(bytes)
		requests[data.reqId] = true
	}
}

func handler(ch chan<- RPC) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		rrIndex += 1
		if rrIndex == len(upstreamHosts) {
			rrIndex = 0
		}
		id := xid.New().String()
		mu.Unlock()
		requests[id] = false
		ch <- RPC{w: w, r: r, reqId: id, hostIdx: rrIndex}
		ticker := time.NewTicker(time.Millisecond * 1)
		for _ = range ticker.C {
			if val := requests[id]; val {
				return
			}
		}
	})
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
