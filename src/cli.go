package src

import (
	"fmt"
	"os"
	"strings"
)

func handleNamedMode(name string, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("missing command after %s", name)
	}
	
	fmt.Println("You are running in named mode")
	fmt.Println("name:", name)
	fmt.Println("cmd:", strings.Join(cmdArgs, " "))

	return nil
}

func cmdList() {
	fmt.Println("list")
}

func Cli() {
	args := os.Args[1:]

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
		if err := handleNamedMode(command, args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		os.Exit(0)
	}
	
}