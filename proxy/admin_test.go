package proxy

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestAddRouteViaAPI(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	h := AdminHandler(rt)

	r := httptest.NewRequest("POST", "/routes", strings.NewReader(`{"hostname": "myapp.localhost", "backend": "http://localhost:3000"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	route, ok := rt.Lookup("myapp.localhost")
	if !ok {
		t.Fatalf("Expected route to be created, got none")
	}
	if route.Backend != "http://localhost:3000" {
		t.Fatalf("Expected backend %q, got %q", "http://localhost:3000", route.Backend)
	}

	// json.Encoder.Encode terminates each value with a newline.
	body := strings.TrimSpace(w.Body.String())
	want := fmt.Sprintf(`{"hostname":"myapp.localhost","backend":"http://localhost:3000","pid":%d}`, os.Getpid())
	if body != want {
		t.Fatalf("Expected body %q, got %q", want, body)
	}
}

func TestAddRouteValidation(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	h := AdminHandler(rt)

	r := httptest.NewRequest("POST", "/routes", strings.NewReader(`{"hostname": "myapp.localhost"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	body := strings.TrimSpace(w.Body.String())
	if body != `{"error":"hostname and backend are required"}` {
		t.Fatalf("Expected body %q, got %q", `{"error":"hostname and backend are required"}`, body)
	}

	r = httptest.NewRequest("POST", "/routes", strings.NewReader(`{"backend": "http://localhost:3000", "force": true}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	body = strings.TrimSpace(w.Body.String())
	if body != `{"error":"hostname and backend are required"}` {
		t.Fatalf("Expected body %q, got %q", `{"error":"hostname and backend are required"}`, body)
	}
}

func TestListRoutesEmpty(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	h := AdminHandler(rt)

	r := httptest.NewRequest("GET", "/routes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := strings.TrimSpace(w.Body.String())
	if body != `[]` {
		t.Fatalf("Expected body %q, got %q", `[]`, body)
	}
}

func TestListRoutesWithData(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	h := AdminHandler(rt)

	tests := []struct {
		hostname string
		backend  string
	}{
		{"myapp.localhost", "http://localhost:3000"},
		{"api.localhost", "http://localhost:4000"},
	}

	for _, tt := range tests {
		if err := rt.AddRoute(tt.hostname, tt.backend, false); err != nil {
			t.Fatalf("AddRoute: %v", err)
		}
	}

	r := httptest.NewRequest("GET", "/routes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.Bytes()
	var got []Route
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal body: %v", err)
	}
	pid := os.Getpid()
	want := []Route{
		{Hostname: "myapp.localhost", Backend: "http://localhost:3000", PID: pid},
		{Hostname: "api.localhost", Backend: "http://localhost:4000", PID: pid},
	}
	slices.SortFunc(got, func(a, b Route) int { return cmp.Compare(a.Hostname, b.Hostname) })
	slices.SortFunc(want, func(a, b Route) int { return cmp.Compare(a.Hostname, b.Hostname) })
	if len(got) != len(want) {
		t.Fatalf("got %d routes, want %d: %s", len(got), len(want), body)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("route %d: got %+v, want %+v (body %s)", i, got[i], want[i], body)
		}
	}

}

func TestRemoveRouteViaAPI(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	h := AdminHandler(rt)

	r := httptest.NewRequest("POST", "/routes", strings.NewReader(`{"hostname": "myapp.localhost", "backend": "http://localhost:3000"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	r = httptest.NewRequest("DELETE", "/routes/myapp.localhost", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	r = httptest.NewRequest("GET", "/routes", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := strings.TrimSpace(w.Body.String())
	if body != `[]` {
		t.Fatalf("Expected body %q, got %q", `[]`, body)
	}

}

func TestRemoveRouteNotFound(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	h := AdminHandler(rt)

	for range 2 {
		r := httptest.NewRequest("DELETE", "/routes/myapp.localhost", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("Expected status %d, got %d", http.StatusNoContent, w.Code)
		}
	}

}

func TestProxyReflectsAdminChanges(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)

	proxyURL, adminURL := startTestServer(t, Config{
		Port:    ephemeralPort(t),
		Impl:    ReverseProxyImpl,
		MaxHops: 2,
	}, rt)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello from backend"))
	}))

	defer backend.Close()

	req, err := http.NewRequest("POST", adminURL+"/routes", strings.NewReader(`{"hostname": "myapp.localhost", "backend": "`+backend.URL+`"}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Host = "myapp.localhost"
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var route Route
	if err := json.Unmarshal(body, &route); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if route.Hostname != "myapp.localhost" || route.Backend != backend.URL || route.PID != os.Getpid() {
		t.Fatalf("Expected hostname %q, backend %q, pid %d, got %+v", "myapp.localhost", backend.URL, os.Getpid(), route)
	}

	req, err = http.NewRequest("GET", proxyURL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Host = "myapp.localhost"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(body, []byte("hello from backend")) {
		t.Fatalf("Expected body %q, got %q", "hello from backend", body)
	}

	req, err = http.NewRequest("DELETE", adminURL+"/routes/myapp.localhost", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	req, err = http.NewRequest("GET", proxyURL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Host = "myapp.localhost"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}
