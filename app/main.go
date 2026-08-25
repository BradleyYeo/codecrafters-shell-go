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
// Output: None (writes directly to stdout/stderr).
func execCommand(input string) {
	if len(input) == 0 {
		return
	}
	fmt.Printf(cmdNotFoundFmt, input)
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
