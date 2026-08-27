package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	promptSymbol    = "$ "
	cmdNotFoundFmt  = "%s: command not found\n"
	typeBuiltinFmt  = "%s is a shell builtin\n"
	typeExecFmt     = "%s is %s\n"
	typeNotFoundFmt = "%s: not found\n"
	typeDirNotFound = "cd: %s: No such file or directory\n"
)

// flowAction specifies the subsequent lifecycle step of the REPL loop.
type flowAction int

const (
	actionContinue flowAction = iota
	actionExit
)

// evalCommand dispatches commands to registered builtins or the external driver.
// Input: structured command, shell execution context.
// Output: flowAction signalling whether the REPL loop continues.
func evalCommand(cmd command, ctx shellContext) flowAction {
	if cmd.name == "" {
		return actionContinue
	}

	if handler, ok := ctx.builtins[cmd.name]; ok {
		return handler(cmd, ctx)
	}

	runExternal(cmd, ctx)
	return actionContinue
}

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

// isBuiltin checks if a command name exists within the injected registry.
func (ctx shellContext) isBuiltin(name string) bool {
	_, ok := ctx.builtins[name]
	return ok
}

// shellContext encapsulates I/O streams to avoid global state dependencies.
type shellContext struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	// builtins acts as the single source of truth for builtin lookup and execution.
	builtins map[string]builtinHandler
}

func main() {
	ctx := shellContext{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		builtins: map[string]builtinHandler{
			"echo": builtinEcho,
			"exit": builtinExit,
			"type": builtinType,
			"pwd":  builtinPwd,
			"cd": builtinCd,
		},
	}

	// Initialize reader once on fd 0 (Standard Input) to preserve buffered stream state (bytes already read from the underlying OS file descriptor via the read syscall).
	reader := bufio.NewReader(ctx.stdin)
	for {
		fmt.Fprint(ctx.stdout, promptSymbol)

		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		cmd := parseCommandLine(strings.TrimSpace(input))
		action := evalCommand(cmd, ctx)
		if action == actionExit {
			break
		}
	}
}
