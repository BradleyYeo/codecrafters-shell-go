package main

import (
	"fmt"
	"bufio"
	"os"
	"strings"
)

const (
	promptSymbol = "$ "
	cmdNotFoundFmt = "%s: command not found\n"
)

// Input: trimmed command string (e.g., "echo hello").
// Output: None (writes directly to stdout/stderr). Exit with code 0 if fail
func execCommand(command string) {
	if command == "" {
		return
	} else if (command == "exit") {
		os.Exit(0)
	} else if (strings.HasPrefix(command, "echo")) {
		args := os.Args[1:]
		output := strings.Join(args, " ")
		fmt.Println(output)
	}
	fmt.Printf(cmdNotFoundFmt, command)
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
