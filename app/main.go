package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	promptSymbol    = "$ "
	cmdNotFoundFmt  = "%s: command not found\n"
	typeBuiltinFmt  = "%s is a shell builtin\n"
	typeExecFmt     = "%s is %s\n"
	typeNotFoundFmt = "%s: not found\n"
)

// command represents a parsed shell invocation.
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

// findExecutable searches the PATH environment variable for a binary.
// Input: binary name (e.g. "ls", "grep").
// Output: absolute path if found, and a boolean indicating success.
func findExecutable(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

var builtins = map[string]struct{}{
	"echo": {},
	"exit": {},
	"type": {},
}

// evalType inspects whether a target command is a shell builtin.
// Input: target command name (e.g. "echo").
// Output: None (writes directly to stdout).
func evalType(command string) {
	if _, ok := builtins[command]; ok {
		fmt.Printf(typeBuiltinFmt, command)
		return
	}

	if path, ok := findExecutable(command); ok {
		fmt.Printf(typeExecFmt, command, path)
		return
	}

	fmt.Printf(typeNotFoundFmt, command)
}

// runExternal executes external binaries by forwarding standard streams.
// Input: structured command with binary name and arguments.
// Output: None (writes directly to OS streams).
func runExternal(cmd command) {
	if _, ok := findExecutable(cmd.name); !ok {
		fmt.Printf(cmdNotFoundFmt, cmd.name)
		return
	}

	proc := exec.Command(cmd.name, cmd.args...)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin

	_ = proc.Run()
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
	case "type":
		if len(cmd.args) > 0 {
			evalType(cmd.args[0])
		}
		return true
	default:
		runExternal(cmd)
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
