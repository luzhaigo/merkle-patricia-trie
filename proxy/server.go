package proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

var targetURL = "http://localhost:8080"

func manualProxyHandler(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest(r.Method, targetURL+r.URL.RequestURI(), r.Body)
	if err != nil {
		log.Printf("proxy error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		if HopByHopHeadersMap[key] {
			continue
		}

		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	if ip, port, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		req.Header.Add(XForwardedForHeader, ip)
		req.Header.Add(XForwardedPortHeader, port)
		host := r.Header.Get("Host")
		req.Header.Add(XForwardedHostHeader, host)
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		req.Header.Add(XForwardedProtoHeader, proto)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway: backend unreachable"))
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		if HopByHopHeadersMap[key] {
			continue
		}

		for _, v := range values {
			w.Header().Add(key, v)
		}

	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func newReverseProxyHandler() http.Handler {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("invalid target URL: %v", err)
	}
	return httputil.NewSingleHostReverseProxy(target)
}

func withHopLimit(maxHops int, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops, err := strconv.Atoi(r.Header.Get(XPortlessHopsHeader))
		if err != nil {
			hops = 0
		}
		if hops >= maxHops {
			w.WriteHeader(http.StatusLoopDetected)
			w.Write([]byte("Loop detected: too many proxy hops"))
			return
		}

		r.Header.Set(XPortlessHopsHeader, strconv.Itoa(hops+1))
		handler.ServeHTTP(w, r)
	})
}

func StartServer(config Config) error {
	var handler http.Handler
	if config.Impl == ManualProxyImpl {
		handler = http.HandlerFunc(manualProxyHandler)
	} else {
		handler = newReverseProxyHandler()
	}

	maxHops := MaxHops
	if config.MaxHops > 0 {
		maxHops = config.MaxHops
	}
	return http.ListenAndServe(":"+strconv.Itoa(config.Port), withHopLimit(maxHops, handler))

}
