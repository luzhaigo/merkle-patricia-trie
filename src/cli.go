package src

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runCommandWithName(name string, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("missing command after %s", name)
	}
	
	fmt.Println("name:", name)
	fmt.Println("cmd:", strings.Join(cmdArgs, " "))

	return nil
}

func cmdList() {
	fmt.Println("list")
}

func runNamedMode(args []string) {
	if len(args) == 0 || args[0] == "help" {
		usage := fmt.Sprintf(`%s %s
Usage:
	portless-go <name> <cmd> [args...]
	portless-go run [--name <name>] <cmd>
	portless-go list
	portless-go help
	portless-go version`, Name, Version)

		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(1)
		} else {
			fmt.Fprintln(os.Stdout, usage)
			os.Exit(0)
		}
	}

	command := args[0]
	switch command {
	case "version":
		fmt.Println(Name, Version)
		os.Exit(0)
	case "list":
		cmdList()
		os.Exit(0)
	default:
		if err := runCommandWithName(command, args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		os.Exit(0)
	}
}

func inferName(dir string) string {
	return filepath.Base(dir)
}

func runRunMode(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing command")
		os.Exit(1)
	}
	
	var name string
	if args[0] != "--name" {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to get current directory")
			os.Exit(1)
		}

		name = inferName(dir)
	} else {
		if len(args) <= 2 {
			if len(args) == 1 {
				fmt.Fprintln(os.Stderr, "missing name value")
				os.Exit(1)
			}

			fmt.Fprintln(os.Stderr, "missing command")
			os.Exit(1)
		}

 		name = args[1]
		args = args[2:]
	}

	if err := runCommandWithName(name, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
	
}

func Cli() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing command")
		os.Exit(1)
	}

	if args[0] == "run" {
		runRunMode(args[1:])
		return 
	}
	
	runNamedMode(args)
	
}