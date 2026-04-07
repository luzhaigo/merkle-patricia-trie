# Phase 4: Route Registration API

## What is route registration?

In Phase 3 you hardcoded routes in `main.go`. That's fine for testing but doesn't match how portless actually works: routes are **registered at runtime** by separate processes (CLI invocations, scripts, etc.) while the proxy is already running.

In upstream portless, running `portless myapp npm start` or `portless alias myapp 3000` **writes to `routes.json`** via the `RouteStore` — there is no HTTP registration API. The proxy picks up new routes by re-reading the file.

For this learning project you'll build **an internal HTTP API** on a separate port so you can add/remove routes with `curl` without restarting the proxy. This is a common pattern in Go services (admin/control plane on a different port from the data plane).

## How upstream portless does it

Read [`packages/portless/src/routes.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/routes.ts) — the `RouteStore` class.

Key things to notice:

1. **`addRoute(hostname, port, pid, force?)`** — acquires the mkdir lock, loads routes (pruning stale PIDs), checks for conflicts (same hostname owned by a live process), optionally kills the existing owner with `--force`, then saves.

2. **`removeRoute(hostname)`** — acquires lock, loads, filters out the hostname, saves.

3. **Conflict detection** — If a hostname is already registered by a **live** process and `force` is false, upstream throws `RouteConflictError`. With `--force`, the existing process gets `SIGTERM` before the route is replaced.

4. **No HTTP API** — Upstream uses direct file manipulation from the CLI process. Your Go version adds an HTTP layer on top of the same `RouteTable` for convenience and learning.

## What you'll build

An internal admin server that:
- Listens on a **separate port** (e.g. `1356` or configurable) from the proxy port (`1355`)
- Exposes endpoints to **add**, **remove**, and **list** routes
- Mutates the same `RouteTable` the proxy uses (so changes take effect immediately for in-memory lookups and are persisted to `routes.json`)
- Clears the proxy cache when routes change so stale reverse proxies don't linger

```
# Add a route:
curl -X POST http://localhost:1356/routes \
  -H "Content-Type: application/json" \
  -d '{"hostname": "myapp.localhost", "backend": "http://localhost:3000"}'

# List routes:
curl http://localhost:1356/routes

# Remove a route:
curl -X DELETE http://localhost:1356/routes/myapp.localhost
```

## Go concepts you'll practice

- **`http.ServeMux`** (Go 1.22+) — register method-specific handlers like `POST /routes` and `DELETE /routes/{name}` with path parameters via `r.PathValue("name")`
- **`encoding/json`** — decode request bodies (`json.NewDecoder(r.Body).Decode(...)`) and encode response bodies (`json.NewEncoder(w).Encode(...)`)
- **Multiple `http.Server` instances** — proxy on one port, admin on another, both in the same process sharing the same `RouteTable`
- **`net/http` status codes** — `201 Created`, `200 OK`, `204 No Content`, `400 Bad Request`, `404 Not Found`, `409 Conflict`
- **`httptest`** — test the admin API without starting a real listener

## Tasks

Work through these in order.

---

### Task 4.1: Build the admin HTTP handler

**What to do:**
- Create `proxy/admin.go`
- Define an `AdminHandler` that takes a `*RouteTable` and returns an `http.Handler` (or `*http.ServeMux`)
- Implement these endpoints:

| Method | Path | Request body | Response | Status |
|--------|------|-------------|----------|--------|
| `POST` | `/routes` | `{"hostname": "...", "backend": "..."}` | `{"hostname": "...", "backend": "...", "pid": ...}` | `201 Created` |
| `GET` | `/routes` | — | `[{"hostname": "...", "backend": "...", "pid": ...}, ...]` | `200 OK` |
| `DELETE` | `/routes/{name}` | — | — | `204 No Content` |

- `POST /routes`:
  1. Decode JSON body into a struct with `Hostname` and `Backend` fields
  2. Validate: both fields must be non-empty; return `400` otherwise
  3. Call `rt.AddRoute(hostname, backend)` — this already sets PID, acquires locks, and persists
  4. Clear the proxy cache so the next request rebuilds handlers for the new backend set
  5. Return `201` with the route as JSON

- `GET /routes`:
  1. Call a new method on `RouteTable` (e.g. `ListRoutes() []Route`) that returns all routes under `RLock`
  2. Encode as JSON array and return `200`

- `DELETE /routes/{name}`:
  1. Extract `name` from the path (Go 1.22+: `r.PathValue("name")`)
  2. Call `rt.RemoveRoute(name)`
  3. Clear the proxy cache
  4. Return `204 No Content`

**How this maps to upstream portless:**

| Node.js (portless) | Go |
|--------------------|-----|
| `store.addRoute(hostname, port, pid)` called directly by CLI | `POST /routes` → `rt.AddRoute(hostname, backend)` |
| `store.removeRoute(hostname)` called directly by CLI | `DELETE /routes/{name}` → `rt.RemoveRoute(name)` |
| `store.loadRoutes()` used by proxy to get current routes | `GET /routes` → `rt.ListRoutes()` |
| No HTTP API (file-based only) | HTTP admin API (learning addition) |

**Hints:**
- Use Go 1.22+ `ServeMux` patterns:

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /routes", handleAddRoute(rt))
mux.HandleFunc("GET /routes", handleListRoutes(rt))
mux.HandleFunc("DELETE /routes/{name}", handleRemoveRoute(rt))
```

- For `ListRoutes`, add to `router.go`:

```go
func (rt *RouteTable) ListRoutes() []Route {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    routes := make([]Route, 0, len(rt.routes))
    for _, r := range rt.routes {
        routes = append(routes, r)
    }
    return routes
}
```

- To clear the proxy cache after mutations, expose a function (e.g. `ClearProxyCache()`) or pass a callback into the admin handler. The simplest approach: call `reloadRoutesAndClearCache` or just reset the cache map directly.

- Set `Content-Type: application/json` on responses:

```go
w.Header().Set("Content-Type", "application/json")
```

**Acceptance criteria:**
- `POST /routes` with valid JSON adds a route and returns `201`
- `POST /routes` with missing fields returns `400`
- `GET /routes` returns all registered routes as a JSON array
- `DELETE /routes/{name}` removes the route and returns `204`
- Routes are persisted to `routes.json` after add/remove
- Proxy cache is cleared after add/remove so routing reflects changes

---

### Task 4.2: Wire the admin server into StartServer / main.go

**What to do:**
- Add an `AdminPort` field to `Config` (default: `1356` or `DefaultPort + 1`)
- In `StartServer` (or in `main.go`), create a **second** `http.Server` for the admin API:

```go
adminMux := AdminHandler(rt)
adminSrv := &http.Server{
    Addr:    ":" + strconv.Itoa(config.AdminPort),
    Handler: adminMux,
}
```

- Start both servers in separate goroutines; shut both down on cleanup
- Remove the hardcoded `AddRoute` calls from `main.go` — routes are now registered via the admin API
- Test with `curl`:

```bash
# Start the proxy (no hardcoded routes):
go run .

# Register routes via the admin API:
curl -X POST http://localhost:1356/routes \
  -H "Content-Type: application/json" \
  -d '{"hostname": "myapp.localhost", "backend": "http://localhost:3000"}'

curl -X POST http://localhost:1356/routes \
  -H "Content-Type: application/json" \
  -d '{"hostname": "api.localhost", "backend": "http://localhost:4000"}'

# Verify routes:
curl http://localhost:1356/routes

# Test routing through the proxy:
curl -H "Host: myapp.localhost" http://localhost:1355/

# Remove a route:
curl -X DELETE http://localhost:1356/routes/myapp.localhost

# Verify it's gone:
curl http://localhost:1356/routes
curl -H "Host: myapp.localhost" http://localhost:1355/   # should be 404 now
```

**Hints:**
- Return both servers from `StartServer`, or return a struct. Alternatively, start the admin server separately in `main.go`.
- Use `log.Printf` to print both addresses on startup so you know which port is which.
- For graceful shutdown, use `signal.NotifyContext` or a `sync.WaitGroup` to wait for both servers.

**Acceptance criteria:**
- Proxy listens on one port, admin API on another
- Routes registered via `POST /routes` are immediately routable through the proxy
- Routes removed via `DELETE` immediately return 404 from the proxy
- No hardcoded routes in `main.go`

---

### Task 4.3: Handle conflicts (optional stretch)

**What to do:**
- Match upstream's conflict detection: if a hostname is already registered by a **live** process (PID check), return `409 Conflict` with a message like `"myapp.localhost is already registered by PID 12345"`.
- Add a `force` query parameter or JSON field: `POST /routes?force=true` or `{"hostname": "...", "backend": "...", "force": true}` that overrides the conflict (like upstream's `--force`).
- Optionally send `SIGTERM` to the existing owner before replacing (like upstream does).

**Acceptance criteria:**
- Adding a route that's already owned by a live process returns `409` without `force`
- Adding with `force=true` replaces the route (and optionally kills the old owner)
- Adding a route whose previous owner is dead succeeds without `force` (stale cleanup)

---

### Task 4.4: Write tests for this phase (mandatory)

**What to do:**
- Create `proxy/admin_test.go` for the admin API tests
- Use `httptest.NewRecorder` + direct handler calls (no need to start a real server for unit tests)

**Admin API tests:**
- `TestAddRouteViaAPI` — POST a route, verify 201, verify `GET /routes` includes it, verify proxy can route to it
- `TestAddRouteValidation` — POST with missing hostname or backend, verify 400
- `TestListRoutesEmpty` — GET with no routes registered, verify empty JSON array `[]`
- `TestListRoutesWithData` — Add routes, GET, verify all are listed
- `TestRemoveRouteViaAPI` — POST a route, DELETE it, verify 204, verify GET no longer includes it
- `TestRemoveRouteNotFound` — DELETE a hostname that doesn't exist (decide: 204 idempotent, or 404)
- `TestProxyReflectsAdminChanges` — Add a route via admin API, send a request through the proxy with that `Host`, verify it reaches the backend; then remove via admin API, verify proxy returns 404

**Hints:**
- For handler-level tests (no real listener):

```go
req := httptest.NewRequest("POST", "/routes", strings.NewReader(`{"hostname":"x.localhost","backend":"http://localhost:9000"}`))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
adminMux.ServeHTTP(w, req)
// assert w.Code == 201
```

- For end-to-end tests that also verify the proxy, reuse `startTestServer` and add the admin server alongside it.
- Use `t.TempDir()` for route files and `t.Parallel()` as before.

**Acceptance criteria:**
- `go test ./proxy/...` passes
- Admin API unit tests cover add, list, remove, validation
- At least one integration test verifies proxy routing reflects admin changes

---

## Useful links

- [`http.ServeMux`](https://pkg.go.dev/net/http#ServeMux) — Go 1.22+ enhanced routing with method and path parameters
- [`json.NewDecoder`](https://pkg.go.dev/encoding/json#NewDecoder) / [`json.NewEncoder`](https://pkg.go.dev/encoding/json#NewEncoder) — streaming JSON encode/decode
- [`httptest.NewRecorder`](https://pkg.go.dev/net/http/httptest#NewRecorder) — test HTTP handlers without a real server
- [`r.PathValue`](https://pkg.go.dev/net/http#Request.PathValue) — extract path parameters (Go 1.22+)
- Upstream routes: [`packages/portless/src/routes.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/routes.ts)

## Node.js vs Go

| Node.js (upstream portless) | Go (this project) |
|-----------------------------|-------------------|
| `store.addRoute(hostname, port, pid)` | `POST /routes` → `rt.AddRoute(hostname, backend)` |
| `store.removeRoute(hostname)` | `DELETE /routes/{name}` → `rt.RemoveRoute(name)` |
| `store.loadRoutes()` | `GET /routes` → `rt.ListRoutes()` |
| Direct file manipulation from CLI | HTTP admin API (learning addition) + same file persistence |
| `RouteConflictError` + `--force` | `409 Conflict` + `?force=true` (optional stretch) |
| No separate admin server | Admin server on separate port from proxy |

The main architectural difference: upstream doesn't need an HTTP registration API because the CLI process directly manipulates `routes.json`. Your Go version adds a thin HTTP layer so you can register routes with `curl` and learn Go's `ServeMux`, JSON handling, and multi-server patterns.

## When you're done

Show me your code and I'll review it. I'll check that:
1. It compiles (`go build ./...`)
2. Tests pass (`go test ./...`)
3. All acceptance criteria are met
4. Admin API correctly adds/removes/lists routes
5. Proxy immediately reflects admin changes (cache cleared)
6. Routes persist to and load from `routes.json`
7. No hardcoded routes remain in `main.go`

Then we'll check off Phase 4 and move to Phase 5 (child process spawning with PORT injection).
