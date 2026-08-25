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

// Input: trimmed command string (e.g., "echo hello").
// Output: None (writes directly to stdout/stderr). Exit with code 0 if fail
func execCommand(command string) {
	if command == "" {
		return
	}

	parts := strings.Split(command, " ")
	cmdName := parts[0]
	args := parts[1:]

	switch cmdName {
	case "exit":
		os.Exit(0)
	case "echo":
		fmt.Println(strings.Join(args, " "))
	default:
		fmt.Printf(cmdNotFoundFmt, command)
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
		command := strings.TrimSpace(input)
		execCommand(command)
	}
}
