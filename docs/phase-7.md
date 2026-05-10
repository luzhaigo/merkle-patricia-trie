# Phase 7: `list` Command and Polish

## Goal

Wrap up the MVP. By the end of this phase the user can:

```bash
portless-go list
# HOSTNAME              BACKEND                 PID    STATUS
# myapp.localhost       http://localhost:4123   12345  alive
# api.localhost         http://localhost:4555   23456  dead
```

…see what's running, kill it cleanly with Ctrl+C, and read a `README.md` that actually explains how to use the tool. Plus you'll do an idiomatic-Go pass with `golangci-lint` so the codebase reads like Go and not "Go that someone learning Go wrote on a Sunday".

## What's already in place

- **`proxy.AdminHandler`** already exposes `GET /routes` returning the table as JSON. You don't need to add a server-side endpoint.
- **`proxy/client.go`** has `RegisterRoute` and `DeregisterRoute` taking an `adminBaseURL`. Phase 7 adds the third sibling: `ListRoutes`.
- **`cli.ParseOptions.OnList`** is wired in `main.go` but is a stub — `log.Println("list (not wired to admin API yet)")`. You'll replace it with a real client call.
- **`startProxy`** already does graceful shutdown on SIGINT/SIGTERM via `signal.NotifyContext` and `srv.Shutdown(ctx)`. Phase 7 just audits the corners.

## How upstream portless does it

Read [`packages/portless/src/cli.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli.ts) — search for `list` / `handleList`. Upstream:

1. **Reads `routes.json` directly** through `RouteStore.list()` — no HTTP call, since upstream registers via direct file access too.
2. **Filters out dead processes** — for each route, if the recorded PID is no longer alive, it's either pruned or marked stale.
3. **Pretty-prints** as a table with hostname, backend port, PID, and uptime.

Your Go version queries the running proxy over the admin API instead. Same idea, different transport — and consistent with the choice you made in Phase 6 (avoid the two-process file-vs-memory split).

## What you'll build

### `proxy.ListRoutes(adminBaseURL string) ([]Route, error)`

A thin client wrapper around `GET /routes`. Returns the slice of routes the admin API serves, or a wrapped error if the proxy isn't reachable.

### `OnList` wired in `main.go`

Replace the stub with:

1. Call `proxy.ListRoutes(resolveAdminBaseURL())`.
2. If the proxy isn't reachable, print a friendly message ("is the proxy running? try `portless-go`") and exit 1.
3. Otherwise pretty-print to stdout — aligned columns, plus a "no routes" note when the slice is empty.

### `routeProcessAlive` exposed for the CLI

Whether a route's PID is still alive is useful information for `list`. The check already lives in `proxy/router.go` (private). Decide whether to:

- **Surface it server-side** by including a `"status"` field in the JSON response, OR
- **Keep `Route` clean** and let the client re-derive status from the PID it sees.

Either is defensible; the first is simpler for now.

### Graceful shutdown audit

Walk the existing `startProxy` shutdown path and answer:

- If the admin server `Shutdown` errors, do you still try the proxy server? (Today: no — it `log.Fatalf`s.)
- Are there any routes left in `routes.json` from a previous crash? Should startup prune dead ones?
- Does Ctrl+C in the spawn process actually deregister the route? (You handled this in Task 6.3, but verify.)

### README refresh

The current `README.md` says "Usage (planned)" — drop that word now that it works. Add a copy-pasteable end-to-end example.

### Idiomatic Go pass

Run `golangci-lint run ./...` and address whatever it finds. Don't blanket-accept every suggestion — for each one, decide if it improves clarity or just shuffles bytes.

## Go concepts you'll practice

- **`net/http` as a client** — `http.Get`, decoding JSON into a slice, distinguishing transport errors from HTTP status errors.
- **`text/tabwriter`** — easy aligned-column output without manual padding.
- **`errors.Is` / sentinel errors** — distinguishing "proxy not running" from generic failures so the CLI can show a useful message.
- **`golangci-lint` workflow** — reading lint output, deciding which suggestions to accept, configuring `.golangci.yml` to silence false positives.
- **README authoring** — runnable examples, scope statements, install instructions.

## Tasks

---

### Task 7.1: `ListRoutes` admin client

**What to do:**

Add to **`proxy/client.go`**:

```go
func ListRoutes(adminBaseURL string) ([]Route, error)
```

**Steps:**

1. `url.JoinPath(adminBaseURL, "routes")` — same pattern as `RegisterRoute`.
2. `http.Get(endpoint)` — simpler than building a request manually since there's no body or headers.
3. Defer-close the response body.
4. On non-200, return an error that includes the status. Reuse `parseJSONErrorBody` for the message.
5. On 200, `json.NewDecoder(resp.Body).Decode(&routes)` and return.

**Hints:**

- `Route` already has JSON tags (check `proxy/router.go`). If it doesn't, add them — `json:"hostname"`, `json:"backend"`, `json:"pid"`.
- Consider a sentinel error like `var ErrProxyUnreachable = errors.New("proxy unreachable")` so callers can `errors.Is` to print a friendly hint. Wrap the underlying `*url.Error` from `http.Get` with it.

**Acceptance criteria:**

- `ListRoutes("http://localhost:1356")` returns the slice the admin API has in memory.
- Returns a clear error when the proxy isn't running (connection refused).
- Tests in `proxy/client_test.go` cover both happy path and unreachable path.

---

### Task 7.2: Wire `OnList` in `main.go`

**What to do:**

Replace the stub:

```go
OnList: func() {
    log.Println("list (not wired to admin API yet)")
},
```

with a real implementation that:

1. Calls `proxy.ListRoutes(resolveAdminBaseURL())`.
2. On `ErrProxyUnreachable`, prints something like:
   ```
   proxy not running. start it with: portless-go
   ```
   to stderr and exits with a non-zero code.
3. On success, prints:
   ```
   HOSTNAME              BACKEND                 PID    STATUS
   myapp.localhost       http://localhost:4123   12345  alive
   ```
   using `text/tabwriter` to keep columns aligned.
4. When the slice is empty, prints `no routes registered` (don't print just the header).

**Hints:**

- `OnList` is currently `func()` (no return). To exit non-zero on error, you'll need to either:
  - Promote `OnList` to `func() error` (and wire it through `cli.Parse` like `OnRun`), OR
  - Have the closure call `os.Exit(1)` directly.
  - The first is cleaner. Mirror what you did for `OnRun` in Phase 6.3.
- For `tabwriter`:
  ```go
  w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
  fmt.Fprintln(w, "HOSTNAME\tBACKEND\tPID\tSTATUS")
  for _, r := range routes {
      fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.Hostname, r.Backend, r.PID, r.Status)
  }
  w.Flush()
  ```

**Acceptance criteria:**

- `portless-go list` shows the routes from the running proxy in aligned columns.
- `portless-go list` with no proxy running prints a clear "start it with…" message and exits non-zero.
- Empty table prints `no routes registered`, not just the header.

---

### Task 7.3: Status field (alive / dead)

**What to do:**

Decide where `Status` lives — server or client — and implement it.

**Option A (recommended): server-side**

1. In `proxy/admin.go`'s `GET /routes` handler, build a response type that includes a `Status string` field per route, computed via the existing `routeProcessAlive(route.PID)`.
2. Either define a separate response struct or add an exported `Status` field to `Route` with `json:"status,omitempty"` and populate it in the handler before encoding.
3. Update `proxy/client.go` to decode the status.

**Option B: client-side**

Have `ListRoutes` return `[]Route`, and let `main.go` re-check liveness for each PID. Downside: client needs OS-level signal access, and PIDs from a different machine wouldn't make sense (not relevant today, but a smell).

**Acceptance criteria:**

- `portless-go list` shows `alive` / `dead` per route.
- Tests cover the status field.

---

### Task 7.4: Graceful shutdown audit

**What to do:**

Walk through `startProxy` in `main.go` and `runApp`. For each scenario, decide if the behavior is right and fix what isn't:

1. **Proxy SIGINT**: Both servers `Shutdown` cleanly with the 10-second timeout. ✓
2. **Admin `Shutdown` errors**: Today the proxy `log.Fatalf`s before trying to shut down the main server. Should it always attempt both, log the first error, and exit non-zero?
3. **Stale routes from a crashed previous run**: When `startProxy` reads `routes.json`, should it prune entries whose PIDs are no longer alive?
4. **Spawn process killed by SIGKILL** (can't be trapped): The admin route lingers. Acceptable? Document as known limitation.
5. **`spawner.SpawnCommand`**: Does it forward Ctrl+C to the child quickly enough? Check the existing `signal.NotifyContext` wiring.

**Hints:**

- Stale-route cleanup at startup is a small loop:
  ```go
  for _, r := range rt.ListRoutes() {
      if !routeProcessAlive(r.PID) { _ = rt.RemoveRoute(r.Hostname) }
  }
  ```
  (`routeProcessAlive` is currently unexported; expose it or move the cleanup into `proxy`.)
- For the "always try both shutdowns" pattern, capture both errors with `errors.Join` (Go 1.20+).

**Acceptance criteria:**

- Document each scenario with a short note (in code comments or `docs/phase-7.md` itself once you've decided).
- Stale routes from a previous crash are not visible after restart.

---

### Task 7.5: README walkthrough

**What to do:**

Update `README.md`:

1. Drop the word "(planned)" from the Usage section — the tool works now.
2. Add a real **Example** section that someone can copy-paste and try in two terminals:
   ```bash
   # Terminal 1
   ./portless-go

   # Terminal 2
   ./portless-go demo python3 -m http.server
   curl -H "Host: demo.localhost" http://localhost:1355/
   ```
3. Add a brief **Architecture** section pointing at the four packages (`cli/`, `proxy/`, `spawner/`, `src/`) and what each does in one sentence.
4. Add an **Environment variables** section (`PORT`, `ADMIN_PORT`, `IMPL`, `MAX_HOPS`).
5. Cross-link to `LEARNING_TODO.md` and the phase docs.

**Acceptance criteria:**

- A reader who has never seen this repo can clone, build, and run a real example using only the README.
- All commands in the README are correct and tested.

---

### Task 7.6: Idiomatic Go pass with `golangci-lint`

**What to do:**

1. Make sure `golangci-lint` is on `PATH` (via `brew install golangci-lint` or `go install` + `$(go env GOPATH)/bin` on `PATH`).
2. Drop a minimal `.golangci.yml` at the repo root:
   ```yaml
   linters:
     enable:
       - errcheck
       - errorlint
       - govet
       - staticcheck
       - revive
       - unused
       - misspell
   ```
3. Run `golangci-lint run ./...` and triage the output.
4. For each finding: fix it, or add an inline `//nolint:<linter>` with a one-line reason if it's a false positive.

**Hints:**

- Common ones you'll likely see:
  - `errorlint` — replace `err == http.ErrServerClosed` with `errors.Is(err, http.ErrServerClosed)` (you've already done these).
  - `errcheck` — handle or explicitly discard return values you're ignoring (`_ = something()`).
  - `revive` — variable naming, exported symbol comments.
  - `staticcheck` — simplification suggestions, deprecated APIs.
- Don't fix everything in one giant commit. Group related fixes (one commit per linter or per package) so the diff is reviewable.

**Acceptance criteria:**

- `golangci-lint run ./...` passes with zero warnings.
- Where a warning is suppressed, there is an inline comment explaining why.

---

### Task 7.7: Tests for this phase

**What to do:**

Add to **`proxy/client_test.go`**:

- `TestListRoutes` — register a route, then `ListRoutes`, verify the slice contains it with the right fields (including `Status` if you went with Option A in Task 7.3).
- `TestListRoutesEmpty` — empty table, verify `ListRoutes` returns an empty slice (not nil, not an error).
- `TestListRoutesProxyUnreachable` — bind/close pattern from `TestRegisterRouteConnectionRefused`. Verify the returned error satisfies `errors.Is(err, ErrProxyUnreachable)` if you introduced that sentinel.

**Optional:**

- `main_test.go` with a `TestMain` that does a real subprocess invocation (`go run . list` against an httptest admin server). Useful for catching CLI wiring regressions, but more setup than the spec requires.

**Acceptance criteria:**

- `go test ./...` passes.
- Client tests cover the happy path, empty path, and unreachable path.

---

## Useful links

- [`text/tabwriter`](https://pkg.go.dev/text/tabwriter) — aligned-column output.
- [`net/http#Get`](https://pkg.go.dev/net/http#Get) — quick GETs without building a request.
- [`errors.Join`](https://pkg.go.dev/errors#Join) — combine multiple shutdown errors.
- [`errors.Is`](https://pkg.go.dev/errors#Is) — sentinel error matching.
- [`golangci-lint` docs](https://golangci-lint.run/usage/configuration/) — `.golangci.yml` reference.
- Upstream CLI: [`packages/portless/src/cli.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli.ts) (`handleList`).

## Node.js vs Go

| Node.js (upstream portless) | Go (this project) |
|-----------------------------|-------------------|
| `RouteStore.list()` reads `routes.json` directly | `proxy.ListRoutes(adminBaseURL)` queries admin API |
| Uses `chalk` and table libraries for colored output | `text/tabwriter` (no third-party deps) |
| Process liveness check via `process.kill(pid, 0)` | `routeProcessAlive(pid)` calls `p.Signal(syscall.Signal(0))` |
| Pretty-prints uptime in addition to status | Out of scope for this phase (requires storing `StartedAt`) |

## When you're done

Show me your code and I'll check that:

1. `go build ./...` compiles, `go test ./...` passes.
2. `golangci-lint run ./...` is clean (or each suppression is justified).
3. `portless-go list` against a running proxy prints aligned columns with status.
4. `portless-go list` against no proxy prints a helpful message and exits non-zero.
5. README has a copy-pasteable end-to-end example that works.
6. Stale routes from a previous crashed run don't survive a restart.

Then we'll mark Phase 7 complete and the MVP is done — at that point the project is "real portless, smaller, in Go". Worth taking a step back and reading through the whole codebase top-to-bottom; you'll be surprised how much you've internalized about Go's standard library by then.
