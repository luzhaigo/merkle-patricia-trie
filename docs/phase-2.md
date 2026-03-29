# Phase 2: HTTP Server and Reverse Proxy Basics

## Goal

Build an HTTP server that listens on port 1355 and forwards all incoming requests to a backend. When this phase is done, you'll have a working reverse proxy — requests come in on one port and get forwarded to another. No routing by hostname yet (that's Phase 3).

## How upstream portless does it

Read [`packages/portless/src/proxy.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/proxy.ts) in the upstream repo.

Key things to notice:

1. **The proxy listens on port 1355** (defined in [`cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts) as `DEFAULT_PROXY_PORT = 1355`). Configurable via `PORTLESS_PORT` env var or `--port` flag.

2. **Forwarding is manual** — portless does NOT use a third-party proxy library. It creates an `http.request(...)` to `127.0.0.1:<backendPort>` and pipes the request/response bodies:

```javascript
const proxyReq = http.request({
  hostname: "127.0.0.1",
  port: route.port,
  path: req.url,
  method: req.method,
  headers: proxyReqHeaders,
}, (proxyRes) => {
  res.writeHead(proxyRes.statusCode, responseHeaders);
  proxyRes.pipe(res);
});
req.pipe(proxyReq);
```

3. **The server is created with** `http.createServer(handleRequest)` — a single handler function processes every request.

The Go equivalent is much simpler thanks to `httputil.ReverseProxy`, which handles the request forwarding, header copying, and body streaming for you.

## What you'll build

A reverse proxy that:
- Listens on port 1355 (or a configurable port)
- Forwards all requests to a single hardcoded backend
- Copies response status, headers, and body back to the client

```
Browser → localhost:1355 → proxy → localhost:BACKEND_PORT → your app
```

## Go concepts you'll practice

- **`net/http`** — creating an HTTP server with `http.ListenAndServe` or `http.Server`
- **`net/http/httputil.ReverseProxy`** — Go's built-in reverse proxy
- **`net/url.Parse`** — parsing URLs for the proxy target
- **`http.Handler` interface** — the core abstraction: anything with `ServeHTTP(w, r)` can handle requests
- **`net/http/httptest`** — spinning up test servers without binding real ports

## Tasks

Work through these in order.

---

### Task 2.1: A simple HTTP server

**What to do:**
- Create a new file (you can start in `src/` or create a `proxy/` directory — your choice)
- Write an HTTP server that listens on a port and responds with a simple message like "portless-go proxy running"
- Make the port configurable (accept it as a function parameter, default to 1355)

**Hints:**
- `http.ListenAndServe(":1355", handler)` is the simplest way to start a server
- A handler is anything that implements `http.Handler` — or you can use `http.HandlerFunc` to wrap a plain function
- The function signature for a handler is `func(w http.ResponseWriter, r *http.Request)`

**Acceptance criteria:**
- Run the server, `curl http://localhost:1355` → get a response
- The port is not hardcoded in the function (passed as a parameter)

---

### Task 2.2: Forward requests to a backend

**What to do:**
- Use `httputil.NewSingleHostReverseProxy(targetURL)` to create a reverse proxy
- The target URL is the backend (e.g. `http://localhost:8080`)
- For now, hardcode the backend URL — we'll make it dynamic in Phase 3
- Replace your placeholder handler from Task 2.1 with the reverse proxy

**How `httputil.ReverseProxy` works:**
- You give it a target URL
- It implements `http.Handler` — so you can pass it directly to your server
- For every incoming request, it rewrites the URL to point at the target and forwards the request
- It copies the response (status, headers, body) back to the original client

**Hints:**
- `url.Parse("http://localhost:8080")` creates the target URL
- `httputil.NewSingleHostReverseProxy(target)` returns an `*httputil.ReverseProxy` which is an `http.Handler`
- To test: start a simple backend (e.g. `python3 -m http.server 8080`), then start your proxy, then `curl http://localhost:1355`

**Acceptance criteria:**
- Start a backend on any port (e.g. 8080)
- Start your proxy on 1355 pointing at the backend
- `curl http://localhost:1355` returns the backend's response
- `curl http://localhost:1355/some/path` forwards the path correctly

---

### Task 2.3: Extract into a `proxy/` package

**What to do:**
- Create a `proxy/` directory
- Move your proxy server code into `proxy/server.go` with `package proxy`
- Export a function like `func StartServer(listenAddr string, targetURL string) error` that wires up the reverse proxy and starts listening
- Update your CLI or `main.go` to call the proxy package

**Why extract now?** The proxy is a separate concern from the CLI. Putting it in its own package enforces clean boundaries — the CLI decides *what* to proxy, the proxy package handles *how*.

**Hints:**
- Exported names in Go start with an uppercase letter
- The package name should match the directory name: `proxy/server.go` → `package proxy`
- Import it from your CLI as `"portless-go/proxy"`
- Keep the proxy server logic self-contained — it shouldn't know about CLI arguments

**Acceptance criteria:**
- `proxy/server.go` exists with `package proxy`
- The proxy can be started by calling a function from the package
- Everything still works: `curl http://localhost:1355` forwards to the backend

---

### Task 2.4: Write tests for this phase

**What to do (mandatory):**
- Create `proxy/server_test.go`
- Use `httptest.NewServer` to create a fake backend that returns a known response
- Start your proxy pointing at the fake backend
- Make HTTP requests to the proxy and verify the response matches the backend

**Go testing tools for HTTP:**
- `httptest.NewServer(handler)` — spins up a real HTTP server on a random port, returns its URL
- `httptest.NewRecorder()` — captures what a handler writes without a real server
- `http.Get(url)` — make a simple GET request

**Example pattern:**

```go
func TestProxyForwards(t *testing.T) {
    // 1. Create a fake backend
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("hello from backend"))
    }))
    defer backend.Close()

    // 2. Create your proxy pointing at the backend
    // ... (use your proxy setup code with backend.URL as the target)

    // 3. Make a request to the proxy
    // ... 

    // 4. Verify the response
    // ...
}
```

**Acceptance criteria:**
- `go test ./proxy/...` passes
- At least 2 tests: basic forwarding works, and path is preserved

---

## Useful links

- [`net/http` package](https://pkg.go.dev/net/http) — HTTP server and client
- [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy) — built-in reverse proxy
- [`httputil.NewSingleHostReverseProxy`](https://pkg.go.dev/net/http/httputil#NewSingleHostReverseProxy) — convenience constructor
- [`httptest` package](https://pkg.go.dev/net/http/httptest) — test utilities for HTTP
- [`net/url.Parse`](https://pkg.go.dev/net/url#Parse) — URL parsing
- Upstream proxy: [`packages/portless/src/proxy.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/proxy.ts)
- Upstream port config: [`packages/portless/src/cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts)

## Node.js vs Go

| Node.js (upstream portless) | Go equivalent |
|-----------------------------|---------------|
| `http.createServer(handler)` | `http.Server{Handler: handler}` or `http.ListenAndServe` |
| `http.request({hostname, port, path, method, headers}, cb)` + `pipe` | `httputil.ReverseProxy` does this for you |
| `req.pipe(proxyReq)` / `proxyRes.pipe(res)` | Handled internally by `ReverseProxy` (uses `io.Copy`) |
| `handler(req, res)` | `ServeHTTP(w http.ResponseWriter, r *http.Request)` |

The big difference: upstream portless builds the proxy **by hand** (manual `http.request` + pipe). In Go, `httputil.ReverseProxy` gives you all of that in one struct. This is a case where Go's standard library is actually **more convenient** than Node's.

## When you're done

Show me your code and I'll review it. I'll check that:
1. It compiles (`go build ./...`)
2. Tests pass (`go test ./...`)
3. All acceptance criteria are met
4. Code follows Go conventions
5. The proxy package is cleanly separated from the CLI

Then we'll check off Phase 2 and move to Phase 3 (host-based routing).
