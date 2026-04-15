# Phase 6: Wiring It All Together

## Goal

Turn the **separate pieces** (CLI parsing, proxy server, admin API, route table, child-process spawner) into a **single unified tool** where one command does everything:

```bash
portless-go myapp npm start
#  1. Ensure the proxy is running (or start it)
#  2. Find a free port
#  3. Register myapp.localhost → http://localhost:<port>
#  4. Spawn "npm start" with PORT=<port>
#  5. On exit → deregister the route
```

By the end of this phase, `portless-go` works like upstream `portless`: a proxy daemon running in one terminal (or auto-started) and any number of app commands spawned from other terminals, each registering and cleaning up its own route.

## How upstream portless does it

Read [`packages/portless/src/cli.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli.ts) — three key pieces:

### `main()` — CLI dispatch

```
portless run <cmd>         → handleRunMode  (infer name from directory)
portless <name> <cmd>      → handleNamedMode (explicit name)
portless proxy start       → start daemon
portless list              → show active routes
portless --help / --version
```

Both `handleRunMode` and `handleNamedMode` call `runApp(...)` with the resolved name and command.

### `runApp(...)` — the orchestrator

1. Check if the proxy daemon is already running (`isProxyRunning`); if not, auto-start it.
2. `parseHostname(name, tld)` → e.g. `myapp.localhost`.
3. `findFreePort()` → ephemeral port.
4. `store.addRoute(hostname, port, pid, force)` → register in `routes.json`.
5. `spawnCommand(commandArgs, { env: { PORT, HOST, PORTLESS_URL }, onCleanup })` → start child.
6. `onCleanup` → `store.removeRoute(hostname)`.

### Route registration is **direct file access**, not HTTP

Upstream never calls an HTTP admin API — it writes `routes.json` directly via `RouteStore`. Your Go implementation **also** has direct `AddRoute`/`RemoveRoute` on `RouteTable`, plus the HTTP admin API from Phase 4.

**For Phase 6**, your spawn process will register routes **via the admin API** so the running proxy process picks up changes immediately (avoiding the two-process file-vs-memory problem you saw in Phase 5 smoke testing). This is a pragmatic simplification; Phase 7 or later could add file-watching for parity with upstream.

## What you'll build

### Refactored `main.go` with proper CLI dispatch

```
portless-go                          → start proxy daemon
portless-go <name> <cmd> [args...]   → spawn mode (named)
portless-go run [--name <n>] <cmd>   → spawn mode (infer or --name)
portless-go list                     → show routes
portless-go help / version           → info
```

### A `runApp` function (orchestration core)

Located in `main.go` or a new `cmd/` file:

```go
func runApp(name string, cmdArgs []string) error
```

Steps:
1. Resolve proxy address (default `localhost:1355`, or from env).
2. Find a free port via `spawner.FindFreePort`.
3. Register the route via admin API: `POST http://localhost:<adminPort>/routes`.
4. Spawn the child with `spawner.SpawnCommand`, injecting `PORT=<port>`.
5. `result.Wait()` — blocks until child exits.
6. Deregister the route via admin API: `DELETE http://localhost:<adminPort>/routes/<hostname>`.
7. Propagate exit code.

### Admin API client helper

A small function (in `proxy/` or a new `client/` package) to talk to the admin API:

```go
func RegisterRoute(adminAddr, hostname, backend string) error
func DeregisterRoute(adminAddr, hostname string) error
```

This keeps `runApp` clean and testable.

## Go concepts you'll practice

- **Refactoring `main.go`** — extracting functions, separating concerns
- **`net/http` as a client** — `http.Post`, `http.NewRequest` + `http.DefaultClient.Do`
- **`path.Base` / `os.Getwd`** — infer project name from directory
- **`os.Exit` with child exit code** — propagate `result.ExitCode()` to the shell
- **Integration between packages** — `proxy`, `spawner`, `src` (CLI) working together

## Tasks

---

### Task 6.1: Admin API client

**What to do:**

Create `proxy/client.go` with helper functions:

```go
package proxy

func RegisterRoute(adminAddr, hostname, backend string) error
func DeregisterRoute(adminAddr, hostname string) error
```

**`RegisterRoute`:**
1. Build a `POST` request to `http://<adminAddr>/routes` with JSON body `{"hostname": "<hostname>", "backend": "<backend>"}`.
2. Send it with `http.DefaultClient.Do(req)`.
3. Check response status: `201` = success; `409` = conflict (return the error message); else return the status.
4. Close the body.

**`DeregisterRoute`:**
1. Build a `DELETE` request to `http://<adminAddr>/routes/<hostname>`.
2. Send it.
3. Check response status: `204` = success; else return the status/body as error.

**Hints:**

```go
body, _ := json.Marshal(addRouteRequest{Hostname: hostname, Backend: backend})
req, _ := http.NewRequest("POST", "http://"+adminAddr+"/routes", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
resp, err := http.DefaultClient.Do(req)
```

**Acceptance criteria:**
- `RegisterRoute("localhost:1356", "myapp.localhost", "http://localhost:4000")` sends a valid POST and parses the response.
- `DeregisterRoute("localhost:1356", "myapp.localhost")` sends DELETE and returns nil on 204.
- Errors from the server (409, 500, connection refused) are returned as Go errors.

---

### Task 6.2: Refactor CLI dispatch in `main.go`

**What to do:**

Replace the current `if len(os.Args) <= 1 { … } else { … }` with proper subcommand dispatch. You can reuse and adapt the CLI parsing from `src/cli.go` (Phase 1).

```go
func main() {
    args := os.Args[1:]

    if len(args) == 0 {
        startProxy()
        return
    }

    switch args[0] {
    case "help", "--help", "-h":
        printUsage()
    case "version", "--version", "-v":
        printVersion()
    case "list":
        listRoutes()
    case "run":
        // parse --name flag, infer name from dir if absent
        name, cmdArgs := parseRunArgs(args[1:])
        runApp(name, cmdArgs)
    default:
        // Named mode: portless-go <name> <cmd> [args...]
        name := args[0]
        if len(args) < 2 {
            fmt.Fprintf(os.Stderr, "Usage: portless-go <name> <cmd> [args...]\n")
            os.Exit(1)
        }
        runApp(name, args[1:])
    }
}
```

**`startProxy()`** — extracted from the current proxy-start branch. Reads env for ports, creates `RouteTable`, starts both servers, waits for signal, shuts down.

**`listRoutes()`** — `GET http://localhost:<adminPort>/routes`, pretty-print the JSON.

**`printUsage()` / `printVersion()`** — reuse or adapt from `src/cli.go`.

**Hints:**
- For `run` mode name inference:

```go
func inferName() string {
    dir, err := os.Getwd()
    if err != nil {
        log.Fatal(err)
    }
    return filepath.Base(dir)
}
```

- For `parseRunArgs`:

```go
func parseRunArgs(args []string) (name string, cmdArgs []string) {
    if len(args) >= 2 && args[0] == "--name" {
        return args[1], args[2:]
    }
    return inferName(), args
}
```

**Acceptance criteria:**
- `portless-go` (no args) starts the proxy daemon.
- `portless-go myapp sh -c 'echo $PORT'` runs in named mode.
- `portless-go run npm start` infers name from directory.
- `portless-go run --name foo npm start` uses `foo` as the name.
- `portless-go list` shows routes from the running proxy.
- `portless-go help` / `portless-go version` print info.

---

### Task 6.3: Implement `runApp`

**What to do:**

Write the orchestration function that ties spawner + admin client together:

```go
func runApp(name string, cmdArgs []string) {
    hostname := name + ".localhost"
    adminAddr := fmt.Sprintf("localhost:%d", adminPort())

    port, err := spawner.FindFreePort(4000, 4999)
    if err != nil { log.Fatalf("find free port: %v", err) }

    backend := fmt.Sprintf("http://localhost:%d", port)
    log.Printf("%s -> %s (PORT=%d)", hostname, backend, port)

    if err := proxy.RegisterRoute(adminAddr, hostname, backend); err != nil {
        log.Fatalf("register route: %v", err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    result, err := spawner.SpawnCommand(ctx, cmdArgs,
        []string{fmt.Sprintf("PORT=%d", port)}, os.Stdout, os.Stderr)
    if err != nil { log.Fatalf("spawn: %v", err) }

    log.Printf("Started PID %d", result.PID)
    _ = result.Wait()

    if err := proxy.DeregisterRoute(adminAddr, hostname); err != nil {
        log.Printf("warning: deregister route: %v", err)
    }

    os.Exit(result.ExitCode())
}
```

**Key decisions:**

- **Admin port**: derive from env (`ADMIN_PORT`) or default (`proxy.DefaultPort + 1`). Use a helper `adminPort() int` so it's consistent with the proxy startup.
- **Exit code propagation**: `os.Exit(result.ExitCode())` so the caller's shell sees the child's status.
- **Deregister on error path**: even if `Wait` returns error (child crashed), still deregister. Use the same pattern upstream uses in `onCleanup`.
- **Deregister is best-effort**: if the proxy is already down, log a warning rather than fatal.

**How this maps to upstream:**

| Upstream `runApp` | Your `runApp` |
|---|---|
| `store.addRoute(hostname, port, pid, force)` | `proxy.RegisterRoute(adminAddr, hostname, backend)` |
| `findFreePort()` | `spawner.FindFreePort(4000, 4999)` |
| `spawnCommand(cmd, { env, onCleanup })` | `spawner.SpawnCommand(ctx, cmd, env, stdout, stderr)` + deregister after Wait |
| `onCleanup → store.removeRoute(hostname)` | `proxy.DeregisterRoute(adminAddr, hostname)` |
| `process.exit(code)` | `os.Exit(result.ExitCode())` |

**Acceptance criteria:**
- Running `portless-go myapp sh -c 'echo PORT=$PORT && sleep 2'` prints the assigned port, then deregisters when the child exits.
- While the child is running, `curl -H "Host: myapp.localhost" http://localhost:1355/` is proxied.
- Ctrl+C sends SIGTERM to the child and deregisters the route.
- The parent exits with the child's exit code.

---

### Task 6.4: End-to-end manual test

**What to do:**

Verify the full flow works with two terminals:

```bash
# Terminal 1 — start the proxy
go run .
# Output: Server listening on :1355, Admin listening on :1356

# Terminal 2 — run an app through portless
go run . myapp sh -c 'exec python3 -m http.server "$PORT"'
# Output: myapp.localhost -> http://localhost:4123 (PORT=4123)
#         Started PID 12345

# Terminal 3 — test it
curl -H "Host: myapp.localhost" http://localhost:1355/
# Should see python's directory listing

# Check routes
go run . list
# Should show myapp.localhost

# Ctrl+C in terminal 2 — child stops, route deregistered
go run . list
# Should be empty (or missing myapp.localhost)
```

Also test `run` mode:

```bash
# From any project directory:
cd ~/my-project
go run ~/workspace/portless-go run sh -c 'echo PORT=$PORT'
# Should infer name "my-project", register my-project.localhost
```

**Acceptance criteria:**
- All the above steps work without errors.
- Route is visible via `list` while child is running.
- Route disappears after child exits or is killed.
- `run` mode infers the correct directory name.

---

### Task 6.5: Write tests

**What to do:**

Add tests that verify the wiring without requiring a live proxy. Focus on:

**Admin client tests** (`proxy/client_test.go`):
- `TestRegisterRoute` — start an `httptest.Server` with `AdminHandler`, call `RegisterRoute`, verify route was added.
- `TestDeregisterRoute` — register then deregister, verify route is gone.
- `TestRegisterRouteConflict` — register twice without force, verify error.
- `TestRegisterRouteConnectionRefused` — call with a bad address, verify a clear error.

**CLI dispatch tests** (optional, in `main_test.go` or via `go run` + subprocess):
- Verify `parseRunArgs` returns correct name and command args.
- Verify `inferName` returns the directory basename.

**Hints:**

For client tests, reuse `httptest`:

```go
rt := proxy.NewRouteTable(filepath.Join(t.TempDir(), "routes.json"))
mux := proxy.AdminHandler(rt)
srv := httptest.NewServer(mux)
defer srv.Close()

// srv.Listener.Addr().String() is the admin address
err := proxy.RegisterRoute(srv.Listener.Addr().String(), "test.localhost", "http://localhost:9000")
```

**Acceptance criteria:**
- `go test ./...` passes.
- Client tests cover success and error paths.
- No test requires a real proxy running.

---

## Useful links

- [`net/http` client](https://pkg.go.dev/net/http) — `http.NewRequest`, `http.DefaultClient`, `http.Post`
- [`os.Getwd`](https://pkg.go.dev/os#Getwd) — current working directory
- [`filepath.Base`](https://pkg.go.dev/path/filepath#Base) — extract directory name
- [`os.Exit`](https://pkg.go.dev/os#Exit) — exit with code (does not run defers)
- [`signal.NotifyContext`](https://pkg.go.dev/os/signal#NotifyContext) — context cancelled on signal
- Upstream CLI: [`packages/portless/src/cli.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli.ts)
- Upstream utils: [`packages/portless/src/cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts)

## Node.js vs Go

| Node.js (upstream portless) | Go (this project) |
|-----------------------------|-------------------|
| `main()` → `handleRunMode` / `handleNamedMode` → `runApp` | `main()` → `switch` dispatch → `runApp` |
| `store.addRoute(hostname, port, pid)` — direct file | `proxy.RegisterRoute(adminAddr, hostname, backend)` — HTTP to admin |
| `spawnCommand(cmd, { env, onCleanup })` | `spawner.SpawnCommand(ctx, cmd, env, stdout, stderr)` + deregister |
| `inferProjectName()` from package.json / directory | `filepath.Base(os.Getwd())` |
| `discoverState()` to find running proxy | Check admin API reachability or assume default ports |
| `process.exit(code)` | `os.Exit(result.ExitCode())` |
| `parseHostname(name, tld)` validates DNS labels | Simple `name + ".localhost"` (extend later if needed) |

The main architectural difference: upstream auto-starts the proxy daemon if it's not running, registers routes via direct file access, and watches for file changes. Your Go version assumes the proxy is already running and registers routes via the admin HTTP API — simpler for now, with room to add daemon management in Phase 7.

## When you're done

Show me your code and I'll check that:

1. `go build ./...` compiles.
2. `go test ./...` passes.
3. `portless-go` (no args) starts the proxy.
4. `portless-go myapp <cmd>` spawns the child, registers the route, and cleans up on exit.
5. `portless-go run <cmd>` infers the name and works identically.
6. `portless-go list` shows active routes.
7. Routes are visible through the proxy while the child runs and gone after it exits.
8. Exit code propagation works (`sh -c 'exit 42'` → parent exits 42).

Then we'll check off Phase 6 and move to Phase 7 (`list` command and polish).
