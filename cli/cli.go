package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"portless-go/src"
)

func printUsage() {
	fmt.Printf(`%s %s
Usage:
	portless-go <name> <cmd> [args...]
	portless-go run [--name <name>] <cmd>
	portless-go list
	portless-go help
	portless-go version`, src.Name, src.Version)
}

func printVersion() {
	fmt.Printf(`%s %s`, src.Name, src.Version)
}

// ParseOptions wires behavior for each dispatch path. Callers (e.g. main) set
// the handlers they support; nil handlers produce errMissingCommand where required.
type ParseOptions struct {
	OnDefault func()
	OnList    func()
	OnRun     func(name string, cmdArgs []string) (int, error)
}

var errMissingCommand = errors.New("missing command")

// ExitUsage is the exit code returned for command-line misuse (missing args,
// unknown subcommand, missing handler, etc.). Follows the bash convention of
// exit 2 for argument errors.
const ExitUsage = 2

func runIfSet(cmd func()) (int, error) {
	if cmd != nil {
		cmd()
		return 0, nil
	}
	return ExitUsage, errMissingCommand
}

func runOnRunIfSet(onRun func(string, []string) (int, error), name string, cmdArgs []string) (int, error) {
	if onRun == nil {
		return ExitUsage, errMissingCommand
	}
	return onRun(name, cmdArgs)
}

// inferName returns the last path segment (used for run-mode default app name).
func inferName(dir string) string {
	return filepath.Base(dir)
}

func parseRunArgs(args []string) (name string, cmdArgs []string, err error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("get working directory: %w", err)
	}

	name = inferName(dir)

	if len(args) >= 2 && args[0] == "--name" {
		name = args[1]
		args = args[2:]
	}

	return name, args, nil
}

// primaryCommand maps aliases to a single key for the handler map.
func primaryCommand(arg0 string) string {
	switch arg0 {
	case "--help", "-h":
		return "help"
	case "--version", "-v":
		return "version"
	default:
		return arg0
	}
}

// topLevel maps normalized first-token commands to handlers. Each handler receives
// the full argv slice (including the command name at args[0]).
var topLevel = map[string]func(args []string, opts ParseOptions) (int, error){
	"help": func(args []string, opts ParseOptions) (int, error) {
		_ = args
		_ = opts
		printUsage()
		return 0, nil
	},
	"version": func(args []string, opts ParseOptions) (int, error) {
		_ = args
		_ = opts
		printVersion()
		return 0, nil
	},
	"list": func(args []string, opts ParseOptions) (int, error) {
		_ = args
		return runIfSet(opts.OnList)
	},
	"run": func(args []string, opts ParseOptions) (int, error) {
		if len(args) < 2 {
			return ExitUsage, fmt.Errorf("run requires a command (e.g. portless-go run npm start)")
		}
		name, cmdArgs, err := parseRunArgs(args[1:])
		if err != nil {
			return 1, err
		}
		if len(cmdArgs) == 0 {
			return ExitUsage, fmt.Errorf("missing command after run")
		}
		return runOnRunIfSet(opts.OnRun, name, cmdArgs)
	},
}

// Parse reads os.Args[1:] and dispatches using opts.
func Parse(opts ParseOptions) (int, error) {
	return parseProgramArgs(os.Args[1:], opts)
}

// parseProgramArgs is Parse with an explicit argv slice (tests use this; Parse delegates here).
func parseProgramArgs(args []string, opts ParseOptions) (int, error) {
	if len(args) == 0 {
		return runIfSet(opts.OnDefault)
	}

	key := primaryCommand(args[0])
	if h, ok := topLevel[key]; ok {
		return h(args, opts)
	}

	// Named mode: portless-go <name> <cmd> [args...]
	if len(args) < 2 {
		return ExitUsage, fmt.Errorf("usage: portless-go <name> <cmd> [args...]")
	}
	return runOnRunIfSet(opts.OnRun, args[0], args[1:])
}
