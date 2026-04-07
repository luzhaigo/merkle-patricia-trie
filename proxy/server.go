package proxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"portless-go/src"
	"strconv"
	"sync"
)

func isBackendUnreachable(err error) bool {
	// Treat common network-layer failures as "unreachable". ReverseProxy can also
	// call ErrorHandler for other cases (e.g. client cancel), so keep this narrow.
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

func reloadRoutesAndClearCache(rt *RouteTable) {
	if err := rt.Load(); err != nil {
		log.Printf("reload route table: %v", err)
		return
	}
	// Minimal cache consistency: after a reload, drop cached handlers so the next
	// request rebuilds proxies for the current backend set.
	cacheMu.Lock()
	cache = make(map[string]http.Handler)
	cacheMu.Unlock()
}

func manualProxyForBackend(backend string, rt *RouteTable) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest(r.Method, backend+r.URL.RequestURI(), r.Body)
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
			if isBackendUnreachable(err) {
				reloadRoutesAndClearCache(rt)
			}
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
}

func manualProxyFor(rt *RouteTable) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := getHost(r.Host)

		route, ok := rt.Lookup(host)
		if !ok {
			http.Error(w, "No app registered for "+host, http.StatusNotFound)
			return
		}

		backend := route.Backend
		h, ok := lookupCache(backend)
		if ok {
			h.ServeHTTP(w, r)
			return
		}

		cacheMu.Lock()
		// Double-check under lock to avoid duplicate work.
		h, ok = cache[backend]
		if !ok {
			h = manualProxyForBackend(backend, rt)
			cache[backend] = h
		}
		cacheMu.Unlock()
		h.ServeHTTP(w, r)
	}
}

func getHost(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}

	return host
}

func lookupCache(backend string) (http.Handler, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	h, ok := cache[backend]
	return h, ok

}

func reverseProxyFor(rt *RouteTable) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := getHost(r.Host)

		route, ok := rt.Lookup(host)
		if !ok {
			http.Error(w, "No app registered for "+host, http.StatusNotFound)
			return
		}

		backend := route.Backend
		h, ok := lookupCache(backend)
		if ok {
			h.ServeHTTP(w, r)
			return
		}

		target, err := url.Parse(backend)
		if err != nil {
			http.Error(w, "Invalid backend URL: "+err.Error(), http.StatusInternalServerError)
			return
		}

		cacheMu.Lock()
		// Double-check under lock to avoid duplicate work.
		h, ok = cache[backend]
		if !ok {
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				http.Error(w, "Bad Gateway: backend unreachable", http.StatusBadGateway)
				if isBackendUnreachable(err) {
					reloadRoutesAndClearCache(rt)
				}
			}
			h = proxy
			cache[backend] = h
		}
		cacheMu.Unlock()
		h.ServeHTTP(w, r)
	})
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

func GetRoutesFilePath() string {
	segments := []string{".", src.Name, "routes.json"}

	if userConfigDir, err := os.UserConfigDir(); err == nil {
		segments[0] = userConfigDir
	} else if wd, err := os.Getwd(); err == nil {
		segments[0] = wd
	}

	return filepath.Join(segments...)
}

var cache = make(map[string]http.Handler)
var cacheMu sync.RWMutex

func ClearProxyCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache = make(map[string]http.Handler)
}

func StartServer(config Config, rt *RouteTable) (*http.Server, error) {
	if err := rt.Load(); err != nil {
		return nil, err
	}

	if config.Port == 0 {
		config.Port = DefaultPort
	}

	var handler http.Handler
	if config.Impl == ManualProxyImpl {
		handler = manualProxyFor(rt)
	} else {
		handler = reverseProxyFor(rt)
	}

	maxHops := MaxHops
	if config.MaxHops > 0 {
		maxHops = config.MaxHops
	}

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(config.Port),
		Handler: withHopLimit(maxHops, handler),
	}

	return srv, nil
}
