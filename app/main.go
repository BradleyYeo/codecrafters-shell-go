package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	promptSymbol   = "$ "
	cmdNotFoundFmt = "%s: command not found\n"
)

// Command is a parsed shell invocation
type command struct {
	name string
	args []string
}

// parseCommandLine tokenizes a raw input line into a structured command.
// Input: raw string from stdin (e.g., "echo foo bar").
// Output: command struct containing the binary/builtin name and arguments.
func parseCommandLine(line string) command {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return command{}
	}

	return command{
		name: parts[0],
		args: parts[1:],
	}
}

// evalCommand executes a parsed command against builtins or external tools.
// Input: structured command.
// Output: boolean indicating whether the shell REPL should continue running.
func evalCommand(cmd command) bool {
	if cmd.name == "" {
		return true
	}

	switch cmd.name {
	case "exit":
		return false
	case "echo":
		fmt.Println(strings.Join(cmd.args, " "))
		return true
	default:
		fmt.Printf(cmdNotFoundFmt, cmd.name)
		return true
	}
}

func main() {
	// Initialize reader once on fd 0 to preserve buffered stream state.
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf(promptSymbol)

		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cmd := parseCommandLine(strings.TrimSpace(input))
		shouldContinue := evalCommand(cmd)
		if !shouldContinue {
			break
		}
	}
}
