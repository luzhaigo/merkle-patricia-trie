package src

import (
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
)

func StartServer(port int) error {
	return http.ListenAndServe(":"+strconv.Itoa(port), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest(r.Method, "http://localhost:8080"+r.URL.RequestURI(), r.Body)
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

		hops := r.Header.Get(XPortlessHopsHeader)
		hopsInt := 0
		if hopsInt, err = strconv.Atoi(hops); err == nil {
			hopsInt++
		}
		req.Header.Set(XPortlessHopsHeader, strconv.Itoa(hopsInt))

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

	}))

}
