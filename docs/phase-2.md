# Phase 2: HTTP Server and Reverse Proxy Basics

## What is a reverse proxy?

A **reverse proxy** sits between clients (browsers) and backend servers. The client talks to the proxy, the proxy talks to the backend, and the client never connects to the backend directly.

```
Without proxy:    Browser ──→ localhost:3000 (your app)
With proxy:       Browser ──→ localhost:1355 (proxy) ──→ localhost:3000 (your app)
```

**Why does portless need one?**

The whole point of portless is to give your app a name like `myapp.localhost` instead of a port number. The proxy is what makes this work — it listens on a single known port (1355), reads the `Host` header to figure out which app you want, and forwards the request to the right backend. Without the reverse proxy, there's no portless.

**What a reverse proxy does (at minimum):**

1. **Accepts incoming connections** on a known port
2. **Forwards the request** (method, path, headers, body) to the backend
3. **Returns the response** (status code, headers, body) to the client
4. **Modifies headers** — adds `X-Forwarded-For` so the backend knows the real client IP, strips hop-by-hop headers that shouldn't be forwarded
5. **Handles errors** — if the backend is down, returns a 502 Bad Gateway instead of crashing

This phase covers all five. Hostname-based routing (deciding *which* backend to forward to) comes in Phase 3.

## Goal

Build an HTTP server that listens on port 1355 and forwards all incoming requests to a backend. When this phase is done, you'll have a working reverse proxy — requests come in on one port and get forwarded to another. No routing by hostname yet (that's Phase 3).

## How upstream portless does it

Read [`packages/portless/src/proxy.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/proxy.ts) in the upstream repo.

Key things to notice:

1. **The proxy listens on port 1355** (defined in [`cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts) as `DEFAULT_PROXY_PORT = 1355`). Configurable via `PORTLESS_PORT` env var or `--port` flag.

2. **Headers are modified before forwarding** — portless does NOT blindly forward all incoming headers. Before creating the outgoing request, it calls `buildForwardedHeaders(req, route, isTls)` which:
   - **Adds** `X-Forwarded-For` (client IP), `X-Forwarded-Proto` (http/https), `X-Forwarded-Host`, `X-Forwarded-Port`
   - **Increments** `X-Portless-Hops` (for loop detection — you'll implement this in Task 2.2c)
   - **Strips** HTTP/2 pseudo-headers (keys starting with `:` like `:method`, `:path`)
   - **Does NOT forward** the raw incoming headers as-is

3. **Forwarding is manual** — portless does NOT use a third-party proxy library. The full flow is:

```javascript
// Step 1: Build modified headers (NOT the raw incoming headers)
const proxyReqHeaders = buildForwardedHeaders(req, route, isTls);

// Step 2: Create outgoing request with modified headers
const proxyReq = http.request({
  hostname: "127.0.0.1",
  port: route.port,
  path: req.url,
  method: req.method,
  headers: proxyReqHeaders,   // ← modified headers
}, (proxyRes) => {
  // Step 3: Send response back — also filters headers for HTTP/2
  res.writeHead(proxyRes.statusCode, filteredResponseHeaders);
  proxyRes.pipe(res);          // stream response body
});

// Step 4: Stream request body (separate from headers)
req.pipe(proxyReq);
```

   Headers and body are **separate concerns** in HTTP. Headers are sent as config when the request is created (step 2). `pipe` only streams the body bytes (step 4).

   On the response side, portless also strips **hop-by-hop headers** (`Connection`, `Keep-Alive`, `Transfer-Encoding`, etc.) for HTTP/2 clients before piping the response body.

4. **The server is created with** `http.createServer(handleRequest)` — a single handler function processes every request.

The Go equivalent is simpler thanks to `httputil.ReverseProxy`, which handles forwarding, header copying, and body streaming. It also automatically adds `X-Forwarded-For` for you.

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
- **`http.Handler` interface** — the core abstraction: anything with `ServeHTTP(w, r)` can handle requests
- **`http.NewRequest`** — creating outgoing HTTP requests manually
- **`http.Client.Do`** — sending requests and getting responses
- **`io.Copy`** — streaming data between a reader and a writer (how body forwarding works)
- **`net/http/httputil.ReverseProxy`** — Go's built-in reverse proxy (does all the above for you)
- **`net/url.Parse`** — parsing URLs for the proxy target
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

### Task 2.2a: Forward requests — the manual way

Build a reverse proxy **by hand** first, the same way upstream portless does it. This teaches you what's actually happening under the hood.

**What to do:**
- Write an `http.Handler` that, for each incoming request:
  1. Creates a new `http.Request` to the backend using `http.NewRequest`
  2. Copies the method, path, and body (`r.Body`) from the incoming request
  3. Copies headers from the incoming request, but **not blindly** — handle them properly (see below)
  4. Adds an `X-Forwarded-For` header with the client's IP address
  5. Sends it with `http.DefaultClient.Do(proxyReq)` (or create your own `http.Client`)
  6. Copies the response status code, headers, and body back to the client using `io.Copy`
- Hardcode the backend URL for now (e.g. `http://localhost:8080`)

**Header handling — what a real proxy does:**

A proxy shouldn't just blindly copy all headers. Here's what to think about:

- **Copy most headers** — `Content-Type`, `Accept`, `Authorization`, cookies, etc. should be forwarded
- **Add `X-Forwarded-For`** — append the client's IP so the backend knows who the real client is. Get it from `r.RemoteAddr` (strip the port with `net.SplitHostPort`)
- **Skip hop-by-hop headers** — headers like `Connection`, `Keep-Alive`, `Transfer-Encoding`, `Upgrade` are meant for the connection between client and proxy, not between proxy and backend. These shouldn't be forwarded (though for a learning project, skipping just `Connection` is fine)
- On the **response** side, same idea — copy most headers, but skip hop-by-hop ones

This mirrors what upstream portless does in `buildForwardedHeaders()`.

**How this maps to upstream portless:**

| Node.js (portless) | Go (manual) |
|---------------------|-------------|
| `buildForwardedHeaders(req, route, isTls)` | Loop over `r.Header`, copy to `proxyReq.Header`, add `X-Forwarded-For` |
| `http.request({..., headers: proxyReqHeaders}, cb)` | `http.NewRequest(method, targetURL+path, body)` + set headers |
| `req.pipe(proxyReq)` — streams request body | Pass `r.Body` (an `io.Reader`) as the 3rd arg to `http.NewRequest` |
| `proxyRes.pipe(res)` — streams response body | `io.Copy(w, resp.Body)` |
| `res.writeHead(statusCode, filteredHeaders)` | Copy response headers + `w.WriteHeader(statusCode)` |

**Hints:**
- `http.NewRequest(r.Method, targetURL+r.URL.RequestURI(), r.Body)` creates the outgoing request
- To copy headers: loop over `r.Header` with `for key, values := range r.Header { ... }` and set them on `proxyReq.Header`
- To skip hop-by-hop: check if the header key is in a set like `{"Connection": true, "Keep-Alive": true}`
- `r.RemoteAddr` looks like `"127.0.0.1:54321"` — use `net.SplitHostPort` to get just the IP
- Copy response headers before calling `w.WriteHeader()` — once you write the status, headers are sent
- `io.Copy(w, resp.Body)` streams the response body — don't forget `defer resp.Body.Close()`
- This is more code than the `ReverseProxy` approach, but you'll understand exactly what a proxy does

**Error handling:**

What should your proxy do when the backend is unreachable? Upstream portless returns **502 Bad Gateway** with a message like "Could not connect to backend". Your manual proxy should do the same:
- If `client.Do(proxyReq)` returns an error, respond with `http.StatusBadGateway` (502)
- Write a simple error message to the response body so the user knows what went wrong

**Acceptance criteria:**
- Start a backend (e.g. `python3 -m http.server 8080`)
- Start your proxy on 1355, `curl http://localhost:1355` → returns the backend's response
- `curl http://localhost:1355/some/path` → path is forwarded correctly
- `curl -X POST -d "hello" http://localhost:1355/echo` → body is forwarded (if your backend supports it)
- `curl -v http://localhost:1355` → response does NOT contain a `Connection` hop-by-hop header from the backend
- The backend receives an `X-Forwarded-For` header with the client IP
- **Stop the backend**, `curl http://localhost:1355` → returns 502 with an error message (not a panic)

---

### Task 2.2b: Forward requests — using `httputil.ReverseProxy`

Now replace your manual proxy with Go's built-in `httputil.ReverseProxy`. Compare how much less code it is.

**What to do:**
- Use `httputil.NewSingleHostReverseProxy(targetURL)` to create a reverse proxy
- Replace your manual handler from Task 2.2a with the reverse proxy
- It implements `http.Handler`, so you can pass it directly to your server

**How `httputil.ReverseProxy` works:**
- You give it a target URL
- It implements `http.Handler` — so you can pass it directly to your server
- For every incoming request, it rewrites the URL to point at the target and forwards the request
- It copies the response (status, headers, body) back to the original client
- Under the hood, it does everything you did manually in 2.2a:
  - Creates an outgoing request with copied headers
  - Streams the request body
  - **Automatically adds `X-Forwarded-For`** with the client IP
  - **Automatically strips hop-by-hop headers** from the response
  - Streams the response body back
- It has a `Director` function you can customize to modify outgoing request headers (useful later in Phase 3 for host-based routing)

**Hints:**
- `url.Parse("http://localhost:8080")` creates the target URL
- `httputil.NewSingleHostReverseProxy(target)` returns an `*httputil.ReverseProxy` which is an `http.Handler`
- Compare the line count with your manual implementation — all that header handling you wrote is built in
- To verify: `curl -v` your proxy and compare the headers with what your manual version produced

**Acceptance criteria:**
- Same as 2.2a: forwarding works, paths preserved, response returned correctly
- Your manual implementation from 2.2a can be kept as a comment or separate file for reference

---

### Task 2.2c: Loop detection with `X-Portless-Hops`

**The problem:** When a frontend dev server (e.g. Vite) proxies API requests to another portless app, it can accidentally create an infinite loop if it doesn't rewrite the `Host` header. The request goes: browser → proxy → frontend → proxy → frontend → proxy → ... forever.

Upstream portless solves this with a hop counter header. Each time the proxy forwards a request, it increments `X-Portless-Hops`. If it exceeds a maximum (5 in upstream), the proxy responds with **508 Loop Detected** instead of forwarding.

**What to do:**
- Before forwarding a request, read the `X-Portless-Hops` header (it's a number as a string, default 0 if missing)
- If the value is >= your max (use 5, matching upstream), respond with **508 Loop Detected** and a message explaining the loop
- Otherwise, increment the value by 1 and set it on the outgoing request
- This works with either your manual proxy (2.2a) or `ReverseProxy` (2.2b)

**How upstream does it** (in `proxy.ts`):

```javascript
const hops = parseInt(req.headers["x-portless-hops"] || "0", 10);
if (hops >= MAX_PROXY_HOPS) {  // MAX_PROXY_HOPS = 5
  res.writeHead(508);
  res.end("Loop detected...");
  return;
}
proxyReqHeaders["x-portless-hops"] = String(hops + 1);
```

**Hints:**
- `r.Header.Get("X-Portless-Hops")` returns the header value as a string (empty if not present)
- `strconv.Atoi(value)` converts a string to int — handle the error by defaulting to 0
- For `ReverseProxy`, you can add this check either in middleware (a handler that wraps the proxy) or in the `Director` function
- HTTP status 508 doesn't have a constant in Go's `net/http` — just use the integer `508`

**Acceptance criteria:**
- Normal requests work as before (hop count starts at 0, gets incremented)
- `curl -H "X-Portless-Hops: 4" http://localhost:1355` → forwarded (hops = 4, under limit)
- `curl -H "X-Portless-Hops: 5" http://localhost:1355` → 508 response with loop message
- `curl -v http://localhost:1355` → response shows `X-Portless-Hops: 1` was sent to backend

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
- At least 4 tests: basic forwarding, path preserved, backend-down returns 502, and hop limit returns 508

---

## Useful links

- [`net/http` package](https://pkg.go.dev/net/http) — HTTP server and client
- [`http.NewRequest`](https://pkg.go.dev/net/http#NewRequest) — create an outgoing request
- [`http.Client`](https://pkg.go.dev/net/http#Client) — send requests and receive responses
- [`io.Copy`](https://pkg.go.dev/io#Copy) — stream data between reader and writer
- [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy) — built-in reverse proxy
- [`httputil.NewSingleHostReverseProxy`](https://pkg.go.dev/net/http/httputil#NewSingleHostReverseProxy) — convenience constructor
- [`httptest` package](https://pkg.go.dev/net/http/httptest) — test utilities for HTTP
- [`net/url.Parse`](https://pkg.go.dev/net/url#Parse) — URL parsing
- Upstream proxy: [`packages/portless/src/proxy.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/proxy.ts)
- Upstream port config: [`packages/portless/src/cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts)

## Node.js vs Go

| Node.js (upstream portless) | Go manual (Task 2.2a) | Go with ReverseProxy (Task 2.2b) |
|-----------------------------|----------------------|----------------------------------|
| `http.createServer(handler)` | `http.ListenAndServe(addr, handler)` | Same |
| `http.request({hostname, port, ...}, cb)` | `http.NewRequest(method, url, body)` + `client.Do(req)` | Handled by `ReverseProxy` |
| `req.pipe(proxyReq)` | Pass `r.Body` to `http.NewRequest` | Handled by `ReverseProxy` |
| `proxyRes.pipe(res)` | `io.Copy(w, resp.Body)` | Handled by `ReverseProxy` |
| `res.writeHead(status, headers)` | Copy headers + `w.WriteHeader(status)` | Handled by `ReverseProxy` |

Upstream portless builds the proxy **by hand**. In Task 2.2a you'll do the same in Go so you understand the mechanics. Then in Task 2.2b, you'll see how `httputil.ReverseProxy` wraps all of that into a single struct.

## When you're done

Show me your code and I'll review it. I'll check that:
1. It compiles (`go build ./...`)
2. Tests pass (`go test ./...`)
3. All acceptance criteria are met
4. Code follows Go conventions
5. The proxy package is cleanly separated from the CLI

Then we'll check off Phase 2 and move to Phase 3 (host-based routing).
