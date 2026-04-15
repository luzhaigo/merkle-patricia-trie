package proxy

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// ephemeralPort returns a free TCP port on 127.0.0.1 for this instant.
// Safe for parallel tests when each test uses its own port.
func ephemeralPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func getProxyURL(t *testing.T, addr string) string {
	t.Helper()
	u, err := url.Parse("http://127.0.0.1" + addr)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return u.String()
}

// startTestServer builds the server, runs ListenAndServe in a goroutine, and
// waits until TCP accepts connections. The server is closed on test cleanup.
func startTestServer(t *testing.T, config Config, rt *RouteTable) (proxyURL, adminURL string) {
	t.Helper()

	// Do not use Port+1 for admin under t.Parallel(): another test's proxy may
	// already use that port. Pick a separate ephemeral port when unset.
	if config.AdminPort == 0 {
		config.AdminPort = ephemeralPort(t)
		for config.AdminPort == config.Port {
			config.AdminPort = ephemeralPort(t)
		}
	}

	srv, adminSrv, err := StartServer(config, rt)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("proxy ListenAndServe: %v", err)
		}
	}()
	go func() {
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("admin ListenAndServe: %v", err)
		}
	}()

	t.Cleanup(func() {
		// Use Errorf, not Fatalf: FailNow in cleanup can skip the second Close.
		if err := srv.Close(); err != nil {
			t.Errorf("proxy Close: %v", err)
		}
		if err := adminSrv.Close(); err != nil {
			t.Errorf("admin Close: %v", err)
		}
	})

	proxyURL = getProxyURL(t, srv.Addr)
	adminURL = getProxyURL(t, adminSrv.Addr)
	waitUntilServing(t, proxyURL)
	waitUntilServing(t, adminURL)

	return proxyURL, adminURL
}

func waitUntilServing(t *testing.T, baseURL string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("server did not become ready in time")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRoutingByHost(t *testing.T) {
	t.Parallel()

	myappBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/test" {
			w.Write([]byte("hello from backend from /test"))
			return
		}
		w.Write([]byte("hello from backend"))
	}))
	// Use Cleanup, not defer: parallel subtests return from t.Run before they finish;
	// defer would close the backend while subtests still run.
	t.Cleanup(func() { myappBackend.Close() })

	apiBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/test" {
			w.Write([]byte("hello from backend from /test"))
			return
		}
		w.Write([]byte("hello from backend"))
	}))
	// Use Cleanup, not defer: parallel subtests return from t.Run before they finish;
	// defer would close the backend while subtests still run.
	t.Cleanup(func() { apiBackend.Close() })

	rt := newRouteTable(t)
	if err := rt.AddRoute("myapp.localhost", myappBackend.URL, false); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}
	if err := rt.AddRoute("api.localhost", apiBackend.URL, false); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}

	proxyURL, _ := startTestServer(t, Config{
		Port:    ephemeralPort(t),
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
	}, rt)

	tests := []struct {
		name     string
		host     string
		path     string
		wantBody string
	}{
		{"root_path", "myapp.localhost", "/", "hello from backend"},
		{"specific_path", "api.localhost", "/test", "hello from backend from /test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest(http.MethodGet, proxyURL+tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Host = tt.host

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if got := string(body); got != tt.wantBody {
				t.Fatalf("body: got %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func newRouteTable(t *testing.T) *RouteTable {
	t.Helper()
	rt := NewRouteTable(t.TempDir() + "/routes.json")
	return rt
}

func Test502WhenBackendDown(t *testing.T) {
	t.Parallel()

	rt := newRouteTable(t)
	rt.AddRoute("myapp.localhost", "http://localhost:3000", false)

	proxyURL, _ := startTestServer(t, Config{
		Port:    ephemeralPort(t),
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
	}, rt)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		host       string
	}{
		{"root_path", "/", http.StatusBadGateway, "myapp.localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest(http.MethodGet, proxyURL+tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Host = tt.host

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", resp.StatusCode, tt.wantStatus)
			}

		})
	}
}

func TestRoutingWithHopLimit(t *testing.T) {
	t.Parallel()

	rt := newRouteTable(t)

	proxyURL, _ := startTestServer(t, Config{
		Port:    ephemeralPort(t),
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
	}, rt)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"root_path", "/", http.StatusLoopDetected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest("GET", proxyURL+tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set(XPortlessHopsHeader, "2")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestUnknownHostReturns404(t *testing.T) {
	t.Parallel()

	rt := newRouteTable(t)

	proxyURL, _ := startTestServer(t, Config{
		Port: ephemeralPort(t),
		Impl: ReverseProxyImpl,
	}, rt)

	req, err := http.NewRequest("GET", proxyURL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Host = "unknown.localhost"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

}
