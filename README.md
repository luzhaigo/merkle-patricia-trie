# portless-go

A Go reimplementation of the **core proxy** from [vercel-labs/portless](https://github.com/vercel-labs/portless) — stable named `.localhost` URLs for local development instead of memorizing port numbers.

This is a **learning project** (my first Go project). The goal is to understand how portless works and practice Go fundamentals by rebuilding the core proxy mechanics. It is not intended for production use or full feature parity with the upstream.

## What It Does

portless-go sits between your browser and your dev server. Instead of accessing `localhost:3000`, you use `myapp.localhost:1355`. When you run:

```bash
portless-go myapp npm start
```

It will:
1. Pick a random port and start `npm start` with `PORT` set to that port
2. Register `myapp.localhost` → the assigned port
3. Proxy HTTP requests from `myapp.localhost:1355` to the dev server

## In Scope

- CLI with `<name> <cmd>`, `run`, and `list` subcommands
- Reverse proxy with Host-based routing
- Child process spawning with `PORT` injection
- In-memory route table

## Out of Scope

HTTPS/TLS, `/etc/hosts` management, git worktree detection, wildcard subdomains, WebSocket proxying, and daemon mode. See the [upstream repo](https://github.com/vercel-labs/portless) for full features.

## Prerequisites

- [Go](https://go.dev/dl/) 1.25.5 or later

## Build & Run

```bash
go build -o portless-go .
./portless-go help
```

## Usage (planned)

```bash
portless-go myapp npm start            # Named mode
portless-go run --name api go run .     # Run mode with explicit name
portless-go run python app.py           # Run mode, infer name from directory
portless-go list                        # Show active routes
```

## Development

```bash
go test ./...       # Run tests
go vet ./...        # Check for issues
```

## Reference

- Upstream: [vercel-labs/portless](https://github.com/vercel-labs/portless) (`packages/portless/`)
- Implementation plan: [`LEARNING_TODO.md`](LEARNING_TODO.md)

## License

This project is for educational purposes.
