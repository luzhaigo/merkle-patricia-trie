# Phase 3: Host-Based Routing

## What is host-based routing?

Right now your proxy forwards **every** request to one hardcoded backend. That's fine for a single app, but the whole point of portless is to manage **multiple** apps at once — each with its own name.

Host-based routing means the proxy reads the `Host` header from each incoming request, looks it up in a **route table**, and forwards to the matching backend. Different hostnames go to different backends:

```
curl http://myapp.localhost:1355/     → proxy → localhost:3000 (myapp)
curl http://api.localhost:1355/       → proxy → localhost:4000 (api)
curl http://unknown.localhost:1355/   → proxy → 404 "No app registered"
```

This is the core of what makes portless useful — one proxy port, many apps.

## How upstream portless does it

Read [`packages/portless/src/proxy.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/proxy.ts), specifically the `handleRequest` function and `findRoute`.

Read [`packages/portless/src/routes.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/routes.ts) for the route storage.

Key things to notice:

1. **Routes are a list of `{hostname, port, pid}` objects.** The proxy receives routes via a `getRoutes()` callback — the route list is external (managed by the `RouteStore` in `routes.ts`). The proxy just reads from it on every request.

2. **Host header parsing:**

```javascript
const host = getRequestHost(req).split(":")[0];
```

The `Host` header looks like `myapp.localhost:1355`. Upstream splits on `:` to get just the hostname (`myapp.localhost`), then looks it up in the route list.

3. **Route lookup with `findRoute`:**

```javascript
function findRoute(routes, host, strict) {
  return (
    routes.find(r => r.hostname === host) ||
    (strict ? undefined : routes.find(r => host.endsWith("." + r.hostname)))
  );
}
```

It tries an **exact match** first. If none, it tries **wildcard subdomain matching** (e.g. `tenant.myapp.localhost` matches `myapp.localhost`). For your learning project, **exact match only** is fine — skip wildcard subdomains.

4. **404 for unknown hosts:** When no route matches, upstream returns a helpful HTML page listing active apps. For your project, a plain text **404 with the unknown hostname** is fine.

5. **Routes live in a file-based store** (`routes.ts`). The `RouteStore` class:
   - Stores routes as a **JSON file** (`routes.json`) on disk
   - Uses **directory-based file locking** (`fs.mkdirSync(lockPath)` — mkdir is atomic on most OS) to prevent concurrent writes from multiple processes
   - **Loads routes** by reading and parsing the JSON file
   - **Saves routes** by writing the JSON file
   - **Cleans up stale routes** by checking if the owning PID is still alive
   - Routes have `{hostname, port, pid}` — the `pid` field tracks which process owns each route

You'll build a similar file-based store in Go in two steps: **Task 3.1a** — JSON persistence, in-process **`sync.RWMutex`**, and **directory-based locking** (`os.Mkdir` on a dedicated lock path, like upstream) so the proxy and other processes (e.g. a future CLI) can share `routes.json` safely. **Task 3.1b** (next) — add `pid` to each route and PID-based stale cleanup like upstream.

### Directory-based locking vs file-based locking

Both solve **“only one writer at a time”** across processes, but they are **different mechanisms**:

| | **Directory lock** (portless: `mkdir` on `routes.lock/`) | **File lock** (common: `flock` / `fcntl` on a lock file) |
|---|----------------------------------------------------------|----------------------------------------------------------|
| **Idea** | Creating a directory fails if it already exists → treat that as “lock held”. | Lock a byte range or whole file; OS enforces exclusivity for lockers that participate. |
| **Typical API** | `mkdir` + retry / stale timeout + `rmdir` to release | `flock(2)` (Go: `syscall.Flock` or third-party wrapper), or `fcntl` |
| **Goal** | Mutual exclusion for read-modify-write of `routes.json` | Same goal |
| **Not identical** | Stale lock dirs need a **timeout** (portless removes old lock dirs). Advisory file locks need all writers to **use** `flock`. | Behavior on NFS, crash mid-write, etc. differs by OS and mount options. |

So: **same high-level result** (serialize writers), **not** the same implementation or edge-case behavior. Portless uses **mkdir** to stay dependency-light and portable with plain `fs` APIs. **Match that in Go** with `os.Mkdir` (and `os.Remove` to release) on a lock directory path. Advisory **`flock`** is a different tool ([`golang.org/x/sys/unix.Flock`](https://pkg.go.dev/golang.org/x/sys/unix#Flock)); you are **not** required to use it for Phase 3 if you implement the mkdir lock like upstream.

### PID-based stale cleanup (what upstream does and why)

**Problem 1 — dead processes, live JSON:** A route says “`myapp.localhost` → port 3000” and records **`pid`** of the CLI or child that registered it. If that process **crashes** or is killed without calling `removeRoute`, the JSON still lists the route. The proxy would keep forwarding to a **dead** app.

**Upstream fix:** On `loadRoutes`, filter routes where `pid` is **not** alive (`process.kill(pid, 0)` in Node — “signal 0” checks existence). Optionally **rewrite** `routes.json` with the filtered list when the store already holds the lock, so garbage doesn’t accumulate.

**Problem 2 — stale lock directory:** If a process dies **while** holding the mkdir lock, the lock directory can remain forever. Portless uses a **mtime threshold**: if the lock dir looks older than ~10s, treat it as stale and remove it, then retry.

**Go equivalents:**

- **Is PID alive:** `process, err := os.FindProcess(pid); err == nil && process.Signal(syscall.Signal(0)) == nil` — note: on Unix, `Signal(0)` is a common existence check; Windows differs (you may use platform-specific code or skip PID on Windows for learning).
- **Store `pid`:** Add `PID int` to your `Route` struct; set it to `os.Getpid()` when registering from the owning process.

You'll implement pruning and tests in **Task 3.1b** (right after Task 3.1a in the task list below).

## What you'll build

A file-backed route table and a routing handler that:
- Stores hostname → backend URL mappings in a **JSON file** on disk
- Keeps an **in-memory cache** protected by a **`sync.RWMutex`** for concurrent lookups inside the proxy process
- Uses **directory-based locking** (`mkdir` on a lock path) around every **read or write** of that JSON file so **multiple processes** (proxy plus registrars) cannot corrupt or interleave updates
- Syncs changes (add/remove) to the file
- On each request, reads the `Host` header, strips the port, looks up the route
- Forwards to the matching backend
- Returns 404 for unknown hostnames

```
Browser ──→ myapp.localhost:1355 ──→ proxy ──→ route table lookup ──→ localhost:3000
Browser ──→ api.localhost:1355   ──→ proxy ──→ route table lookup ──→ localhost:4000
Browser ──→ nope.localhost:1355  ──→ proxy ──→ route table lookup ──→ 404
```

## Go concepts you'll practice

- **`sync.RWMutex`** — protects the in-memory map inside **one** process: many concurrent `Lookup`s (`RLock`), exclusive `AddRoute` / `RemoveRoute` / `Load` / `save` paths (`Lock`) together with file I/O ordering you choose
- **`encoding/json`** — marshaling Go structs to JSON and unmarshaling JSON back to structs
- **`os.ReadFile` / `os.WriteFile`** — reading and writing the routes file (always under the directory lock)
- **`os.Mkdir` / `os.MkdirAll` / `os.Remove`** — **directory lock**: `Mkdir` the lock path to **acquire** (expect `ErrExist` while another process holds it — retry with backoff); `Remove` the **empty** lock directory to **release**. Treat **stale** lock dirs like upstream (e.g. **mtime** older than ~10s → remove and retry). Use `MkdirAll` only for parent dirs of the routes file if needed
- **`strings.SplitN`** / **`net.SplitHostPort`** — string manipulation for host header parsing
- **`httputil.ReverseProxy`** — dynamically creating reverse proxies per route lookup
- **`httptest`** — continuing to use test servers for testing routing scenarios
- **`syscall` / `os.Process.Signal`** — (Task 3.1b) PID liveness checks on Unix

## Tasks

Work through these in order.

---

### Task 3.1a: Build a file-backed route table

**What to do:**
- Create `proxy/router.go`
- Define a `Route` struct to represent a single route entry:

```go
type Route struct {
    Hostname string `json:"hostname"`
    Backend  string `json:"backend"`
}
```

- Define a `RouteTable` struct that keeps:
  - An in-memory `map[string]string` (hostname → backend URL) for fast lookups
  - A `sync.RWMutex` to protect concurrent access to that map within the proxy process
  - A file path where routes are persisted as JSON
  - A **lock directory path** used only for cross-process locking (same idea as upstream’s `routes.lock` next to `routes.json` — pick a convention and document it, e.g. `filepath.Join(filepath.Dir(filePath), "routes.lock")`)
- Implement these methods:
  - `AddRoute(hostname, backendURL string) error` — under `mu.Lock()`, acquire the **directory lock**, update map, `save()`, release directory lock
  - `RemoveRoute(hostname string) error` — same pattern
  - `Lookup(hostname string) (backendURL string, ok bool)` — `mu.RLock()` only; reads the in-memory map (no JSON I/O)
  - `Load() error` — under `mu.Lock()`, acquire the **directory lock**, read JSON from disk into the map, release directory lock (call at startup; see hints for ordering vs concurrent writers)
  - `save() error` — write the in-memory map to the JSON file; **caller** must already hold the directory lock (and typically `mu.Lock()`) so the write is not interleaved with another process

**Why both in-memory and file?** The proxy handles many concurrent requests — reading a file on every request would be slow. The in-memory map is the fast path for `Lookup`. The file is the persistence layer so routes survive restarts.

**How this maps to upstream portless:**

| Node.js (portless) | Go |
|--------------------|-----|
| `RouteStore` class in `routes.ts` | `RouteTable` struct in `router.go` |
| `routes.json` file on disk | JSON file at configurable path |
| `loadRoutes()` — read + parse JSON | `Load()` — `os.ReadFile` + `json.Unmarshal` |
| `saveRoutes()` — write JSON | `save()` — `json.Marshal` + `os.WriteFile` |
| `addRoute(hostname, port, pid)` | `AddRoute(hostname, backendURL)` (Task 3.1b adds `pid` like upstream) |
| `removeRoute(hostname)` | `RemoveRoute(hostname)` |
| `findRoute(routes, host)` | `Lookup(hostname)` |
| `acquireLock()` / `releaseLock()` via `mkdir` | Same pattern in Go: `os.Mkdir(lockPath)` to acquire, `os.Remove(lockPath)` when empty to release, with retry + stale-lock handling (see `routes.ts`) |

Upstream uses directory-based file locking (`mkdir` is atomic) because **more than one process** touches `routes.json` (e.g. proxy and registration tooling). **You should do the same:** wrap every **read or write** of the routes JSON with that directory lock. **`sync.RWMutex` is still required** so many goroutines in the **proxy** can `Lookup` safely while one goroutine mutates the map and file; the **mutex does not** protect you against **another process** on the same file — that is what the **mkdir lock** is for.

**Hints:**
- Start with this struct:

```go
type RouteTable struct {
    mu       sync.RWMutex
    routes   map[string]string  // hostname → backend URL
    filePath string
    lockPath string             // empty directory used as cross-process lock (mkdir / rmdir)
}
```

- Constructor (example — you can pass `lockPath` explicitly instead). Use `import "path/filepath"` for `filepath.Join`:

```go
func NewRouteTable(filePath string) *RouteTable {
    lockPath := filepath.Join(filepath.Dir(filePath), "routes.lock")
    return &RouteTable{
        routes:   make(map[string]string),
        filePath: filePath,
        lockPath: lockPath,
    }
}
```

- Implement small helpers, e.g. `acquireDirLock() error` / `releaseDirLock() error`, mirroring upstream’s acquire/release: loop on `os.Mkdir(lockPath, 0o755)` until success or give up; on `os.ErrExist`, sleep briefly or check **stale** lock (directory **mtime** older than ~10s → `os.Remove(lockPath)` and retry). Release with `os.Remove(lockPath)` (directory must be empty).
- For `save()`: convert the map to a `[]Route` slice, then `json.MarshalIndent` and `os.WriteFile` — only while holding the **directory lock**
- For `Load()`: with **directory lock** held, `os.ReadFile`, then `json.Unmarshal` into a `[]Route` slice, then populate the map
- Handle the case where the file doesn't exist yet (first run) — `Load` should return `nil` (empty routes), not an error
- Use `os.IsNotExist(err)` or `errors.Is(err, os.ErrNotExist)` to check for missing files
- **Ordering:** hold `mu.Lock()` for the whole critical section that touches **both** map and file, and acquire the **directory lock** before `os.ReadFile` / `os.WriteFile` so no other process can read/write the JSON in between. (Avoid deadlock: never wait on the directory lock while only holding `RLock`.)
- `Lookup` only needs `mu.RLock()` — it reads the in-memory map, no file I/O

**Acceptance criteria:**
- `RouteTable` struct with `AddRoute`, `RemoveRoute`, `Lookup`, `Load`, `save`
- **Directory lock** (`mkdir` / `Remove`) around every load and save of the JSON file, with stale-lock handling aligned with upstream’s idea (~10s mtime threshold)
- Routes persist to a JSON file
- `Load` reads them back on startup
- `Lookup` returns `("", false)` for unknown hostnames
- File looks like:

```json
[
  {"hostname": "myapp.localhost", "backend": "http://localhost:3000"},
  {"hostname": "api.localhost", "backend": "http://localhost:4000"}
]
```

---

### Task 3.1b: PID field and stale-route cleanup (mandatory)

Do this **after** Task 3.1a — it extends the same `RouteTable` and JSON format.

**What to do:**

- Add **`PID int`** to your `Route` struct with a `json` tag (use `0` or omit in JSON when you don’t care for a one-off test).
- **`AddRoute`:** set `PID` to **`os.Getpid()`** for routes registered by this process (matches “who owns this route”).
- **`Load` or `PruneStale()`:** after loading from disk (or as part of `Load`), **drop** any route whose **PID is not alive**. On Unix, a common check is `os.FindProcess(pid)` plus `process.Signal(syscall.Signal(0)) == nil` (document that Windows differs; for this project you can scope the test to Unix or use build tags).
- If pruning removes rows, **rewrite** `routes.json` while holding the same **`sync.RWMutex` writer lock** and **directory (`mkdir`) lock** you use for `AddRoute` / `RemoveRoute` so the file stays consistent across processes.
- **Test:** register a route with a **fake PID** that cannot exist (e.g. `999999` on Unix), run your prune path, assert that route is **gone** from the table and, if you re-`Load` from file, still gone.

**Why after 3.1a?** You need a working file-backed table first; then you add a field and cleanup logic without mixing two learning goals in one step.

**Acceptance criteria:**

- JSON on disk includes `pid` per route (for routes you add after this task).
- Stale routes (dead PID) are removed by `Load` and/or `PruneStale` and persisted with `save`.
- Unit test covers fake-PID cleanup (e.g. `TestPruneStalePID`).

---

### Task 3.2: Route requests by Host header

**What to do:**
- Write a new handler (or modify your existing proxy handler) that:
  1. Reads the `Host` header from the incoming request
  2. Strips the port (e.g. `myapp.localhost:1355` → `myapp.localhost`)
  3. Looks up the hostname in the `RouteTable`
  4. If found, creates a `ReverseProxy` for that backend and forwards the request
  5. If not found, returns **404** with a message like `"No app registered for myapp.localhost"`
- Wire the routing handler into `StartServer`

**How upstream does the host lookup** (in `proxy.ts`):

```javascript
const host = getRequestHost(req).split(":")[0];
const route = findRoute(routes, host, strict);

if (!route) {
    res.writeHead(404, { "Content-Type": "text/html" });
    res.end(renderPage(404, "Not Found", ...));
    return;
}

// Forward to route.port
const proxyReq = http.request({
    hostname: "127.0.0.1",
    port: route.port,
    ...
});
```

Your Go equivalent:

```go
host := r.Host
if h, _, err := net.SplitHostPort(host); err == nil {
    host = h
}

backend, ok := routeTable.Lookup(host)
if !ok {
    http.Error(w, "No app registered for "+host, http.StatusNotFound)
    return
}

// Forward to backend...
```

**Design decision — cache reverse proxies (required for this phase):**

Do **not** allocate a new `httputil.ReverseProxy` on every request. **Cache** one `ReverseProxy` per distinct backend URL (or per hostname, if each route maps 1:1 to a backend). Typical approaches: a `map[string]*httputil.ReverseProxy` protected by the same `sync.RWMutex` as your routes, or a `sync.Map` keyed by backend URL. **Invalidate or update** the cache when routes change (`AddRoute` / `RemoveRoute` / `Load`) so you never forward to a stale target. This matches how you’d avoid repeated work in a real proxy.

**Hints:**
- `r.Host` gives you the `Host` header value (e.g. `myapp.localhost:1355`)
- `net.SplitHostPort` separates host and port — but it fails if there's no port, so handle both cases
- For `httputil.ReverseProxy`, parse the backend URL with `url.Parse` and use `NewSingleHostReverseProxy`
- The hop limit middleware should still wrap the routing handler
- `StartServer` no longer needs a single `Backend` URL — it uses the route table instead

**Acceptance criteria:**
- Requests with a registered `Host` are forwarded to the correct backend
- Requests with an unknown `Host` get 404
- Multiple routes can be registered, each going to a different backend
- Reverse proxies are **reused** (cached per backend or per host), not recreated on every request; cache stays consistent when routes change
- Hop limit still works

---

### Task 3.3: Wire routes from main.go (temporary)

**What to do:**
- For now, hardcode a couple of routes in `main.go` to test the routing:

```go
rt := proxy.NewRouteTable("/tmp/portless-go/routes.json")
rt.Load()  // load any persisted routes
rt.AddRoute("myapp.localhost", "http://localhost:3000")
rt.AddRoute("api.localhost", "http://localhost:4000")
```

- Pass the route table to `StartServer` (update `Config` or the function signature)
- Test with `curl`:

```bash
# Start two backends:
python3 -m http.server 3000 &
python3 -m http.server 4000 --directory /tmp &

# Start proxy:
./portless-go

# Test routing:
curl -H "Host: myapp.localhost" http://localhost:1355/
curl -H "Host: api.localhost" http://localhost:1355/
curl -H "Host: unknown.localhost" http://localhost:1355/

# Verify persistence:
cat /tmp/portless-go/routes.json
```

**Why hardcode for now?** Phase 4 will add a proper registration API. For this phase, we just need to verify the routing and persistence work.

**Acceptance criteria:**
- Different `Host` headers route to different backends
- Unknown hosts get 404
- Routes are saved to the JSON file
- Everything still works: hop limit, header forwarding, 502 on backend down

---

### Task 3.4: Write tests for this phase (mandatory)

**What to do:**
- Create `proxy/router_test.go` for the route table unit tests
- Add routing integration tests in `proxy/server_test.go`

**Route table tests (unit):**
- `TestAddAndLookup` — add a route, look it up, verify it's found
- `TestLookupUnknown` — look up an unregistered hostname, verify `ok == false`
- `TestRemoveRoute` — add a route, remove it, verify it's gone
- `TestOverwriteRoute` — add a route twice with different backends, verify the latest wins
- `TestPersistence` — add routes, create a new `RouteTable` pointing at the same file, call `Load`, verify routes are there
- `TestLoadMissingFile` — call `Load` on a nonexistent file, verify no error and empty routes
- `TestPruneStalePID` (from Task 3.1b) — fake dead PID removed after prune / `Load`

**Routing integration tests:**
- `TestRoutingByHost` — register two routes, send requests with different `Host` headers, verify each goes to the right backend
- `TestUnknownHostReturns404` — send a request with an unregistered host, verify 404
- `TestRoutingWithHopLimit` — verify hop limit still works with routing

**Hints:**
- For route table tests, use `t.TempDir()` to get a temporary directory per test — Go cleans it up automatically. This avoids test files polluting your project and means parallel tests don't conflict.
- For integration tests, use `httptest.NewServer` for backends (each returning a unique response so you can verify routing)
- Set the `Host` header on test requests: `req.Host = "myapp.localhost"`
- All tests should use `t.Parallel()` and `ephemeralPort`

**Acceptance criteria:**
- `go test ./proxy/...` passes
- Route table unit tests cover add, lookup, remove, overwrite, persistence, missing file, and stale-PID cleanup (Task 3.1b)
- Integration tests verify host-based routing end-to-end

---

## Useful links

- [`sync.RWMutex`](https://pkg.go.dev/sync#RWMutex) — readers-writer mutex
- [`encoding/json`](https://pkg.go.dev/encoding/json) — JSON encoding and decoding
- [`os.ReadFile`](https://pkg.go.dev/os#ReadFile) / [`os.WriteFile`](https://pkg.go.dev/os#WriteFile) — file I/O
- [`os.Mkdir`](https://pkg.go.dev/os#Mkdir) / [`os.MkdirAll`](https://pkg.go.dev/os#MkdirAll) — directory lock (`Mkdir`) and parent dirs for the routes file (`MkdirAll`)
- [`os.Remove`](https://pkg.go.dev/os#Remove) — remove empty lock directory to release
- [`filepath.Join`](https://pkg.go.dev/path/filepath#Join) — build `lockPath` next to `routes.json`
- [`errors.Is`](https://pkg.go.dev/errors#Is) — error comparison (for `os.ErrNotExist`)
- [`net.SplitHostPort`](https://pkg.go.dev/net#SplitHostPort) — parse "host:port" strings
- [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy) — built-in reverse proxy
- [`t.TempDir`](https://pkg.go.dev/testing#T.TempDir) — per-test temporary directory
- Upstream proxy: [`packages/portless/src/proxy.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/proxy.ts)
- Upstream routes: [`packages/portless/src/routes.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/routes.ts)

## Node.js vs Go

| Node.js (upstream portless) | Go |
|-----------------------------|-----|
| `getRoutes()` callback per request | `routeTable.Lookup(host)` |
| `findRoute(routes, host, strict)` | `Lookup` method on `RouteTable` |
| `RouteStore` with JSON file + `mkdir` lock | `RouteTable` with JSON file + `sync.RWMutex` + `mkdir` directory lock |
| `fs.readFileSync` + `JSON.parse` | `os.ReadFile` + `json.Unmarshal` |
| `fs.writeFileSync` + `JSON.stringify` | `json.MarshalIndent` + `os.WriteFile` |
| `req.headers.host.split(":")[0]` | `net.SplitHostPort(r.Host)` |
| `http.request({hostname, port, ...})` | `httputil.NewSingleHostReverseProxy(target)` |

**Same shape as upstream:** the routes file is shared JSON on disk; **directory-based locking** serializes cross-process access; **`sync.RWMutex`** serializes access to the in-process map. The main intentional simplification in this learning repo is usually **exact hostname match** and fewer edge cases — not skipping the mkdir lock if multiple processes can touch the same `routes.json`.

## When you're done

Show me your code and I'll review it. I'll check that:
1. It compiles (`go build ./...`)
2. Tests pass (`go test ./...`)
3. All acceptance criteria are met
4. Route table is goroutine-safe (`sync.RWMutex`) and safe across processes (directory lock on JSON I/O)
5. Routes persist to and load from the JSON file
6. Task 3.1b: `pid` in JSON, stale routes pruned, fake-PID test passes
7. Host header parsing handles edge cases

Then we'll check off Phase 3 and move to Phase 4 (route registration API).
