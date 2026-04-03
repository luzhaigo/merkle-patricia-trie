# Learning TODO: portless-go

Reimplementing the **core proxy** of [vercel-labs/portless](https://github.com/vercel-labs/portless) in Go as a learning project. Each phase builds on the previous one. Complete tasks in order; check them off as you go.

Reference: `packages/portless/` in the upstream monorepo.

---

## Phase 1: Basic CLI skeleton — [Guide](docs/phase-1.md)

Set up argument parsing, subcommand dispatch, and help/version output.

- [x] Parse `os.Args` and dispatch to subcommands: `run`, `list`, `help`, `version`
- [x] Support named mode: `portless-go <name> <cmd> [args...]`
- [x] Support run mode with `--name` flag and directory-based name inference
- [x] Write tests for this phase

**Upstream reference:** `packages/portless/src/cli.ts`

---

## Phase 2: HTTP server and reverse proxy basics — [Guide](docs/phase-2.md)

Learn `net/http` and `httputil.ReverseProxy` by building a simple forwarding proxy.

- [x] Create an HTTP server that listens on a configurable port (default 1355)
- [x] Forward requests manually (`http.NewRequest` + `io.Copy`) with header handling and 502 error responses
- [x] Replace manual proxy with `httputil.ReverseProxy` and compare
- [x] Add loop detection with `X-Portless-Hops` header (508 Loop Detected)
- [x] Extract proxy logic into a `proxy/` package
- [x] Write tests for this phase

**Upstream reference:** `packages/portless/src/proxy.ts`

---

## Phase 3: Host-based routing — [Guide](docs/phase-3.md)

Route requests to different backends based on the `Host` header.

- [ ] Build a file-backed route table (JSON + `sync.RWMutex` + map) mapping hostnames to backend URLs
- [ ] Use directory-based locking (`os.Mkdir` / `os.Remove` on a lock path, retry + stale mtime handling) around every `routes.json` read/write, like upstream
- [ ] Route requests by parsing the `Host` header and looking up the target backend
- [ ] Wire hardcoded routes from `main.go` to test routing and persistence end-to-end
- [ ] Return 404 with a helpful message for unknown hosts
- [ ] Write tests for this phase (including persistence tests)

**Upstream reference:** `packages/portless/src/proxy.ts` (routing logic), `packages/portless/src/routes.ts` (route storage)

---

## Phase 4: Route registration

Provide a way to add and remove routes at runtime.

- [ ] Add an internal HTTP API (e.g. `POST /routes`, `DELETE /routes/:name`) on a separate port
- [ ] Implement `AddRoute` / `RemoveRoute` methods on the route table
- [ ] Write tests for this phase

**Upstream reference:** `packages/portless/src/routes.ts`

---

## Phase 5: Child process spawning with PORT injection

Start a child command and wire it into the proxy.

- [ ] Use `os/exec` to spawn a child process with a `PORT` env var set to a random free port
- [ ] Forward the child's stdout/stderr to the parent's terminal
- [ ] Handle child process exit and propagate signals (SIGINT, SIGTERM)
- [ ] Write a helper to find a free port in a range (default 4000–4999)
- [ ] Write tests for this phase

**Upstream reference:** `packages/portless/src/process.ts`

---

## Phase 6: Wiring it all together

Connect the CLI, proxy, route table, and process spawner into a working tool.

- [ ] `portless-go <name> <cmd>`: spawn child → register `<name>.localhost` → proxy serves traffic
- [ ] `portless-go run <cmd>`: same flow but infer name from directory
- [ ] On child exit, deregister the route and shut down cleanly
- [ ] Manual end-to-end test: run a simple HTTP server through portless-go, curl `<name>.localhost:1355`
- [ ] Write tests for this phase

**Upstream reference:** `packages/portless/src/cli.ts` (orchestration)

---

## Phase 7: `list` command and polish

Finish the MVP with observability and cleanup.

- [ ] Implement `portless-go list` to display active routes (name, backend port, status)
- [ ] Add graceful shutdown on SIGINT/SIGTERM (stop proxy, kill children, clean up routes)
- [ ] Write a short usage guide in `README.md` with examples
- [ ] Review all code for idiomatic Go: naming, error handling
- [ ] Write tests for this phase

---

## Progress

| Phase | Topic                        | Status         |
| ----- | ---------------------------- | -------------- |
| 1     | Basic CLI skeleton           | ✅ Complete     |
| 2     | HTTP server & reverse proxy  | ✅ Complete     |
| 3     | Host-based routing           | ⬜ Not Started  |
| 4     | Route registration           | ⬜ Not Started  |
| 5     | Child process spawning       | ⬜ Not Started  |
| 6     | Wiring it all together       | ⬜ Not Started  |
| 7     | `list` command and polish    | ⬜ Not Started  |
