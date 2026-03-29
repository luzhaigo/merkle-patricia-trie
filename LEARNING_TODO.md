# Learning TODO: portless-go

Reimplementing the **core proxy** of [vercel-labs/portless](https://github.com/vercel-labs/portless) in Go as a learning project. Each phase builds on the previous one. Complete tasks in order; check them off as you go.

Reference: `packages/portless/` in the upstream monorepo.

---

## Phase 1: Basic CLI skeleton

Set up argument parsing, subcommand dispatch, and help/version output.

- [x] Parse `os.Args` and dispatch to subcommands: `run`, `list`, `help`, `version`
- [x] Support named mode: `portless-go <name> <cmd> [args...]`
- [x] Support run mode with `--name` flag and directory-based name inference
- [ ] Add unit tests for argument parsing and `inferName`

**Upstream reference:** `packages/portless/src/cli.ts`

---

## Phase 2: HTTP server and reverse proxy basics

Learn `net/http` and `httputil.ReverseProxy` by building a simple forwarding proxy.

- [ ] Create an HTTP server that listens on a configurable port (default 1355)
- [ ] Forward all requests to a hardcoded backend using `httputil.ReverseProxy`
- [ ] Add tests that start the proxy, hit it with `net/http/httptest`, and verify forwarding
- [ ] Extract proxy logic into a `proxy/` package

**Upstream reference:** `packages/portless/src/proxy.ts`

---

## Phase 3: Host-based routing

Route requests to different backends based on the `Host` header.

- [ ] Build a route table (`sync.Map` or mutex-guarded map) mapping hostnames to backend URLs
- [ ] Parse the `Host` header and look up the target backend
- [ ] Return 404 with a helpful message for unknown hosts
- [ ] Add tests for route matching (exact match, missing host, `.localhost` suffix handling)

**Upstream reference:** `packages/portless/src/proxy.ts` (routing logic)

---

## Phase 4: Route registration

Provide a way to add and remove routes at runtime.

- [ ] Add an internal HTTP API (e.g. `POST /routes`, `DELETE /routes/:name`) on a separate port
- [ ] Implement `AddRoute` / `RemoveRoute` methods on the route table
- [ ] Test concurrent route registration with multiple goroutines

**Upstream reference:** `packages/portless/src/routes.ts`

---

## Phase 5: Child process spawning with PORT injection

Start a child command and wire it into the proxy.

- [ ] Use `os/exec` to spawn a child process with a `PORT` env var set to a random free port
- [ ] Forward the child's stdout/stderr to the parent's terminal
- [ ] Handle child process exit and propagate signals (SIGINT, SIGTERM)
- [ ] Write a helper to find a free port in a configurable range (default 4000–4999)

**Upstream reference:** `packages/portless/src/process.ts`

---

## Phase 6: Wiring it all together

Connect the CLI, proxy, route table, and process spawner into a working tool.

- [ ] `portless-go <name> <cmd>`: spawn child → register `<name>.localhost` → proxy serves traffic
- [ ] `portless-go run <cmd>`: same flow but infer name from directory
- [ ] On child exit, deregister the route and shut down cleanly
- [ ] Manual end-to-end test: run a simple HTTP server through portless-go, curl `<name>.localhost:1355`

**Upstream reference:** `packages/portless/src/cli.ts` (orchestration)

---

## Phase 7: `list` command and polish

Finish the MVP with observability and cleanup.

- [ ] Implement `portless-go list` to display active routes (name, backend port, status)
- [ ] Add graceful shutdown on SIGINT/SIGTERM (stop proxy, kill children, clean up routes)
- [ ] Write a short usage guide in `README.md` with examples
- [ ] Review all code for idiomatic Go: naming, error handling, test coverage

---

## Progress

| Phase | Topic                        | Status         |
| ----- | ---------------------------- | -------------- |
| 1     | Basic CLI skeleton           | 🔶 In Progress |
| 2     | HTTP server & reverse proxy  | ⬜ Not Started  |
| 3     | Host-based routing           | ⬜ Not Started  |
| 4     | Route registration           | ⬜ Not Started  |
| 5     | Child process spawning       | ⬜ Not Started  |
| 6     | Wiring it all together       | ⬜ Not Started  |
| 7     | `list` command and polish    | ⬜ Not Started  |
