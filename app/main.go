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

func execCommand(input string) {
	if len(input) == 0 {
		return
	}
	fmt.Printf(cmdNotFoundFmt, input)
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(promptSymbol)

	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	command := strings.TrimSpace(input)
	execCommand(command)
}
