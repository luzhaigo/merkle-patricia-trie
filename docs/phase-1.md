# Phase 1: Basic CLI Skeleton

## Goal

Build the command-line interface for `portless-go`. When this phase is done, your binary will parse arguments, dispatch to the right handler, and print helpful output — but won't actually proxy anything yet (that comes in Phase 2).

## How upstream portless does it

Read [`packages/portless/src/cli.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli.ts) in the upstream repo. Here's how it works at a high level:

1. It grabs `process.argv.slice(2)` (everything after `node cli.js`)
2. If the first arg is `run`, it shifts it off and enters **run mode**
3. Otherwise it checks if the first arg is a **reserved subcommand** (`list`, `trust`, `proxy`, etc.) and dispatches to a handler
4. If it's not reserved, the first arg is the **app name** and the rest is the child command — this is **named mode**

The reserved subcommand names in upstream are: `run`, `get`, `alias`, `hosts`, `list`, `trust`, `proxy`. We only care about `run` and `list` for our scope.

The key insight: **the first argument decides everything**. It's either a subcommand or an app name.

## What you'll build

Your CLI should handle these invocations:

```
portless-go <name> <cmd> [args...]          # Named mode
portless-go run [--name <name>] <cmd>       # Run mode
portless-go list                             # List routes (stub)
portless-go help                             # Print usage
portless-go version                          # Print version
```

## Go concepts you'll practice

- **`os.Args`** — a `[]string` where index 0 is the binary name, 1+ are arguments
- **`switch` statement** — clean dispatching without if/else chains
- **`fmt.Fprintf`** with `os.Stderr` vs `os.Stdout` — errors go to stderr, normal output to stdout
- **`os.Exit(1)`** — non-zero exit code signals failure
- **`os.Getwd()`** — returns current working directory (for inferring app name)
- **`path/filepath.Base()`** — extracts the last element of a path
- **`strings.Join()`** — joining a slice into a single string

## Tasks

Work through these in order. Each builds on the last.

---

### Task 1.1: Argument parsing and subcommand dispatch

**What to do:**
- Slice `os.Args` to get just the user arguments (skip the binary name)
- Use a `switch` on the first argument to dispatch to different handlers
- Handle `help` / `--help` / `-h`, `version` / `--version` / `-v`, and `list`
- If the first arg doesn't match a reserved command, treat it as an app name (named mode)
- If no arguments are given, print usage to stderr and exit with code 1

**Hints:**
- Start simple: just print what mode you'd enter and what arguments you got
- You can write stub functions that just print a message: `func cmdList() { ... }`
- Look at how upstream checks `args[0]` against known strings — your Go `switch` is equivalent

**Acceptance criteria:**
- `./portless-go` (no args) → prints usage to stderr, exits with code 1
- `./portless-go help` → prints usage to stdout, exits 0
- `./portless-go version` → prints `portless-go 0.1.0`, exits 0
- `./portless-go list` → prints a stub message, exits 0
- `./portless-go myapp npm start` → prints name and command, exits 0

---

### Task 1.2: Named mode

**What to do:**
- When the first arg is not a reserved command, it's the app name
- Everything after the app name is the child command
- If there's a name but no command after it, that's an error

**Hints:**
- `args[0]` is the name, `args[1:]` is the command
- For the error case, print a helpful message to stderr showing what was missing
- Keep it as a separate function: `func handleNamedMode(name string, cmdArgs []string) error`

**Why `error` return?** In idiomatic Go, functions that can fail return an `error`. The `main()` function checks the error, prints it to stderr, and calls `os.Exit(1)`. This keeps the logic testable — your handler doesn't call `os.Exit` directly.

**Acceptance criteria:**
- `./portless-go myapp npm start` → prints `name="myapp" cmd=npm start`
- `./portless-go myapp` → error: missing command after "myapp", exit 1

---

### Task 1.3: Run mode with `--name` and directory inference

**What to do:**
- `portless-go run <cmd>` — infer the app name from the current directory
- `portless-go run --name api <cmd>` — use the explicit name
- If `--name` is given but no value follows, that's an error
- If no command is given after `run` (or after `--name <name>`), that's an error

**How upstream infers the name:** It checks `package.json`, then git root, then falls back to the current directory basename. For us, **just use the directory basename** — that's enough for learning.

**Hints:**
- `os.Getwd()` returns `(string, error)` — always handle the error case
- `filepath.Base("/Users/you/myproject")` returns `"myproject"`
- Consider making a separate function `func inferName(dir string) string` — it takes a path and returns the last component. This makes it easy to test later.
- Check if `args[0] == "--name"` before treating args as the child command

**Acceptance criteria:**
- `./portless-go run npm start` (from a directory called `portless-go`) → prints `name="portless-go" cmd=npm start`
- `./portless-go run --name api go run .` → prints `name="api" cmd=go run .`
- `./portless-go run` → error: missing command, exit 1
- `./portless-go run --name` → error: missing name value, exit 1

---

### Task 1.4: Unit tests

**What to do:**
- Create `main_test.go` in the same package
- Write **table-driven tests** for `inferName` — pass different directory paths and check the output
- Optionally, if you structured your dispatch as a `func run(args []string) error`, you can test that too

**Go testing basics:**
- Test files end in `_test.go` and live in the same directory
- Test functions start with `Test` and take `*testing.T`
- Run with `go test ./...`

**Table-driven test pattern:**

```go
func TestInferName(t *testing.T) {
    tests := []struct {
        dir  string
        want string
    }{
        {"/home/user/myproject", "myproject"},
        // add more cases...
    }

    for _, tt := range tests {
        got := inferName(tt.dir)
        if got != tt.want {
            t.Errorf("inferName(%q) = %q, want %q", tt.dir, got, tt.want)
        }
    }
}
```

**Acceptance criteria:**
- `go test ./...` passes
- At least 3 test cases for `inferName` (normal path, root path, path with trailing slash)

---

## Useful links

- [`os.Args`](https://pkg.go.dev/os#pkg-variables) — command-line arguments
- [`os.Getwd`](https://pkg.go.dev/os#Getwd) — current working directory
- [`path/filepath.Base`](https://pkg.go.dev/path/filepath#Base) — last element of a path
- [`fmt.Fprintf`](https://pkg.go.dev/fmt#Fprintf) — formatted print to any writer
- [`testing` package](https://pkg.go.dev/testing) — Go's built-in test framework
- Upstream CLI: [`packages/portless/src/cli.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/cli.ts)
- Upstream name inference: [`packages/portless/src/auto.ts`](https://github.com/vercel-labs/portless/blob/main/packages/portless/src/auto.ts)

## When you're done

Show me your code and I'll review it. I'll check that:
1. It compiles (`go build ./...`)
2. Tests pass (`go test ./...`)
3. All acceptance criteria are met
4. Code follows Go conventions (error handling, naming, structure)

Then we'll check off Phase 1 and move to Phase 2 (HTTP server and reverse proxy).
