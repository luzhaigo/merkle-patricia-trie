package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

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
func startTestServer(t *testing.T, config Config) (proxyURL string) {
	t.Helper()

	srv, err := StartServer(config)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	t.Cleanup(func() {
		_ = srv.Close()
	})

	proxyURL = getProxyURL(t, srv.Addr)
	waitUntilServing(t, proxyURL)

	return proxyURL
}

func waitUntilServing(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 50 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not become ready in time")
}

func TestProxyForwards(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/test" {
			w.Write([]byte("hello from backend from /test"))
			return
		}
		w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	proxyURL := startTestServer(t, Config{
		Port:    3000,
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
		Backend: backend.URL,
	})

	tests := []struct {
		name     string
		path     string
		wantBody string
	}{
		{"root_path", "/", "hello from backend"},
		{"specific_path", "/test", "hello from backend from /test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(proxyURL + tt.path)
			if err != nil {
				t.Fatalf("Get: %v", err)
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

func Test502WhenBackendDown(t *testing.T) {
	proxyURL := startTestServer(t, Config{
		Port:    3000,
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
		Backend: "http://localhost:8080",
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"root_path", "/", http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(proxyURL + tt.path)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", resp.StatusCode, tt.wantStatus)
			}

		})
	}
}

func TestHopLimit(t *testing.T) {
	proxyURL := startTestServer(t, Config{
		Port:    3000,
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
		Backend: "http://localhost:8080",
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"root_path", "/", http.StatusLoopDetected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
