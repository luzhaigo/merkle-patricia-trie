package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

func manualProxyFor(backend string) http.HandlerFunc {
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

func reverseProxyFor(backend string) (http.Handler, error) {
	target, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(target), nil
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

func StartServer(config Config) (*http.Server, error) {
	if config.Port == 0 {
		config.Port = DefaultPort
	}

	if config.Backend == "" {
		return nil, fmt.Errorf("backend URL is required")
	}

	var handler http.Handler
	if config.Impl == ManualProxyImpl {
		handler = manualProxyFor(config.Backend)
	} else {
		var err error
		handler, err = reverseProxyFor(config.Backend)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL: %w", err)
		}
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
