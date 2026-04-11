# Phase 5: Child Process Spawning with PORT Injection

## What is child process spawning?

When you run `portless myapp npm start`, portless does not just register a route — it **starts `npm start` for you**, injecting a free ephemeral port as the `PORT` environment variable. The child process listens on that port; the proxy maps `myapp.localhost` to it.

This phase builds the same mechanic in Go:

1. Find a free port in a range (default **4000–4999**).
2. Spawn the user's command as a **child process**, with `PORT=<port>` in its environment.
3. Forward the child's **stdout/stderr** to the parent's terminal.
4. On **SIGINT / SIGTERM** (Ctrl+C), signal the child to stop and clean up.
5. When the child **exits on its own**, propagate its exit code to the parent.

## How upstream portless does it

Read [`packages/portless/src/cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts) — two functions:

### `findFreePort(minPort, maxPort)`

```typescript
// tries random ports first; falls back to sequential
const port = minPort + Math.floor(Math.random() * (maxPort - minPort + 1));
// check by creating a net.Server on that port; if it errors -> try another
```

### `spawnCommand(commandArgs, { env, onCleanup })`

```typescript
// spawns via /bin/sh -c on Unix, cmd.exe on Windows
const child = spawn("/bin/sh", ["-c", commandArgs.join(" ")], {
  stdio: "inherit",  // stdout/stderr go straight to terminal
  env,
});

// forward SIGINT / SIGTERM to child; propagate exit code
process.on("SIGINT", () => { child.kill("SIGINT"); process.exit(130); });
child.on("exit", (code, signal) => { process.exit(code ?? 1); });
```

Key things to notice:

1. **`stdio: "inherit"`** — child's stdout/stderr are connected directly to the terminal. No reading and re-printing.
2. **Random-then-sequential port search** — tries several random ports first (less contention in multi-process scenarios), then sweeps sequentially.
3. **Signal forwarding** — SIGINT/SIGTERM on the parent are forwarded to the child; when the child exits, the parent exits with the same code.
4. **`onCleanup` hook** — called after signal or exit to deregister the route from `RouteStore`.

In Go you'll do the same things with `os/exec`.

## What you'll build

### A `spawner` sub-package (or file in `proxy/`)

```
portless-go/
  spawner/
    spawner.go        ← FindFreePort + SpawnCommand
    spawner_test.go   ← tests
```

Alternatively you can add it as `proxy/spawner.go` — either is fine for now.

### `FindFreePort(min, max int) (int, error)`

Returns a free TCP port in `[min, max]`. Strategy: try a few random ports, then sweep sequentially.

### `SpawnCommand(ctx context.Context, args []string, env []string, onExit func(error)) error`

- Builds an `*exec.Cmd` with:
  - **Stdout / Stderr** connected to `os.Stdout` / `os.Stderr` (inherited IO).
  - Extra **env vars** merged with the current process environment.
- Starts the process.
- Watches for **context cancellation** (proxy SIGINT/SIGTERM): sends `SIGTERM` to the child.
- When the child exits, calls **`onExit(err)`** so the caller can deregister the route.

## Go concepts you'll practice

- **`os/exec`** — `exec.Command`, `cmd.Start`, `cmd.Wait`, `cmd.Process.Signal`
- **`net.Listen`** — use `net.Listen("tcp", ":port")` to test whether a port is free (open, immediately close)
- **`os.Environ` / `append`** — merge parent env with new `KEY=value` pairs
- **`cmd.Stdout / cmd.Stderr`** — set to `os.Stdout` / `os.Stderr` for inherited IO
- **`context.Context`** — cancel child when the parent receives a signal
- **`syscall.SIGTERM`** — send to a child process
- **`os.Exit`** — propagate child exit code

## Tasks

---

### Task 5.1: `FindFreePort`

**What to do:**

Create `spawner/spawner.go` with:

```go
package spawner

import (
    "fmt"
    "math/rand/v2"
    "net"
)

const (
    DefaultMinPort = 4000
    DefaultMaxPort = 4999
)

func FindFreePort(min, max int) (int, error) { ... }
```

**Algorithm** (matches upstream):

1. **Random pass**: try `randomAttempts` (e.g. 10) random ports in `[min, max]`.
2. **Sequential pass**: sweep `min` to `max` in order.
3. For each candidate: try **`net.Listen("tcp", ":port")`**, close immediately; if no error → port is free, return it.
4. If nothing works → return an error.

**Hints:**

```go
ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
if err == nil {
    ln.Close()
    return port, nil
}
```

Use **`math/rand/v2`** (Go 1.22+): `rand.IntN(max - min + 1)` for a random int in `[0, max-min]`.

**Acceptance criteria:**
- Returns a port in `[min, max]` that is free at the time of the call.
- Returns an error if no free port is found.
- `min > max` returns an error immediately.

---

### Task 5.2: `SpawnCommand`

**What to do:**

Add to `spawner/spawner.go`:

```go
// SpawnResult is returned from SpawnCommand to allow the caller to
// wait for the process and retrieve its PID.
type SpawnResult struct {
    PID  int
    Wait func() error // blocks until child exits
}

func SpawnCommand(ctx context.Context, args []string, extraEnv []string) (*SpawnResult, error)
```

Steps inside:

1. **Build the command**: `exec.CommandContext(ctx, args[0], args[1:]...)`.
2. **Inherit IO**: `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr`.
3. **Merge env**: `cmd.Env = append(os.Environ(), extraEnv...)`.
4. **Start**: `cmd.Start()`.
5. Return a `SpawnResult` with `PID = cmd.Process.Pid` and `Wait = cmd.Wait`.

**Signal handling / cleanup:**

Callers should drive shutdown via **context cancellation** — when `ctx` is cancelled, `exec.CommandContext` sends **`SIGKILL`** to the child by default. For **`SIGTERM`** (graceful), set **`cmd.Cancel`** (Go 1.20+):

```go
cmd.Cancel = func() error {
    return cmd.Process.Signal(syscall.SIGTERM)
}
cmd.WaitDelay = 10 * time.Second // kill after 10s if SIGTERM isn't handled
```

**Env injection:**

```go
env := append(os.Environ(),
    fmt.Sprintf("PORT=%d", port),
    fmt.Sprintf("PORTLESS_URL=http://%s:%d", hostname, proxyPort),
)
cmd.Env = env
```

**Acceptance criteria:**
- `SpawnCommand` starts the child and returns its PID.
- The child's stdout/stderr appear in the terminal (inherited IO).
- `SpawnResult.Wait()` blocks until the child exits.
- When `ctx` is cancelled, the child receives SIGTERM and exits within `WaitDelay`.

---

### Task 5.3: Wire into `main.go` (manual smoke test)

**What to do:**

Temporarily update `main.go` so that if arguments are given on the command line, it:

1. Calls **`spawner.FindFreePort(4000, 4999)`**.
2. Calls **`rt.AddRoute(name+".localhost", "http://localhost:"+port, false)`** via the admin API or directly.
3. Calls **`spawner.SpawnCommand(ctx, args, []string{"PORT=..."})`**.
4. Calls **`result.Wait()`** (blocking) and removes the route when the child exits.

This will be fully automated in Phase 6. For now, hard-code or pass arguments to validate the flow works end-to-end:

```bash
# terminal 1 — start proxy
go run .

# terminal 2 — spawn a local HTTP server through portless
go run . myapp python3 -m http.server

# curl through proxy:
curl -H "Host: myapp.localhost" http://localhost:1355/
```

**Acceptance criteria:**
- Child process starts with the right `PORT` env var.
- Proxy routes traffic to the child.
- Ctrl+C stops the child and deregisters the route.

---

### Task 5.4: Write tests

**What to do:**

Create `spawner/spawner_test.go`.

**`FindFreePort` tests:**
- `TestFindFreePortInRange` — returned port is in `[min, max]` and is actually free at call time.
- `TestFindFreePortMinMax` — `min == max` returns that port if free.
- `TestFindFreePortInvalidRange` — `min > max` returns an error.
- `TestFindFreePortExhausted` — bind all ports in a tiny range and verify an error is returned (can use `min == max == <occupied port>`).

**`SpawnCommand` tests:**
- `TestSpawnCommandOutput` — spawn `echo hello`, capture stdout, verify it's forwarded (use a pipe / `cmd.Stdout = &buf` in a test variant, or check exit code).
- `TestSpawnCommandExitCode` — spawn `exit 1` (or `sh -c "exit 1"`), verify `Wait()` returns a non-nil error with the right exit code.
- `TestSpawnCommandContextCancel` — spawn `sleep 30`, cancel the context, verify the process stops within a second.

**Hints:**

For exit code:
```go
var exitErr *exec.ExitError
if errors.As(err, &exitErr) {
    exitErr.ExitCode() // e.g. 1
}
```

For output capture in tests, consider a variant of `SpawnCommand` that accepts `io.Writer` for stdout/stderr — or set `cmd.Stdout` to an `os.Pipe` in the test. Alternatively, test only the `SpawnResult.PID` and process state without stdout.

**Acceptance criteria:**
- `go test ./spawner/...` passes.
- Context cancel test completes in under 2 seconds.

---

## Useful links

- [`os/exec`](https://pkg.go.dev/os/exec) — `exec.Command`, `Cmd.Start`, `Cmd.Wait`, `Cmd.Cancel`, `WaitDelay`
- [`exec.ExitError`](https://pkg.go.dev/os/exec#ExitError) — inspect exit code from `Wait`
- [`net.Listen`](https://pkg.go.dev/net#Listen) — use to probe port availability
- [`math/rand/v2`](https://pkg.go.dev/math/rand/v2) — Go 1.22+ random number generation
- [`syscall.SIGTERM`](https://pkg.go.dev/syscall#Signal) — cross-platform signal
- [`cmd.WaitDelay`](https://pkg.go.dev/os/exec#Cmd.WaitDelay) — kill grace period (Go 1.20+)
- Upstream spawn logic: [`packages/portless/src/cli-utils.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli-utils.ts)

## Node.js vs Go

| Node.js (upstream portless) | Go (this project) |
|-----------------------------|-------------------|
| `net.createServer().listen(port)` to check availability | `net.Listen("tcp", ":port")` then close |
| Random-then-sequential port search | Same algorithm |
| `spawn("/bin/sh", ["-c", cmd], { stdio: "inherit" })` | `exec.Command(args[0], args[1:]...)` with `cmd.Stdout = os.Stdout` |
| `process.on("SIGINT", () => child.kill("SIGINT"))` | `cmd.Cancel` + `cmd.WaitDelay` via context cancel |
| `child.on("exit", (code) => process.exit(code))` | `result.Wait()` returns `*exec.ExitError` with `.ExitCode()` |
| `options.onCleanup()` callback removes route | Caller calls `rt.RemoveRoute` after `Wait()` returns |
| `stdio: "inherit"` | `cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr` |

## When you're done

Show me your code and I'll check that:

1. `go build ./...` compiles.
2. `go test ./...` passes.
3. `FindFreePort` returns a usable port in the given range.
4. `SpawnCommand` starts the child with the right env, streams output, and responds to context cancellation.
5. A manual smoke test (`go run . myapp python3 -m http.server` or similar) routes traffic through the proxy.

Then we'll check off Phase 5 and move to Phase 6 (full CLI wiring).
