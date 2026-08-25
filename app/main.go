package main

import (
	"bufio"
	"fmt"
	"io"
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

// flowAction specifies the subsequent lifecycle step of the REPL loop.
type flowAction int

const (
	actionContinue flowAction = iota
	actionExit
)

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

// builtinHandler defines the uniform interface for all builtin routines.
type builtinHandler func(cmd command, ctx shellContext) flowAction

// builtinEcho prints space-delimited arguments to stdout.
// Input: structured command, shell execution context.
// Output: flowAction indicating continuation.
func builtinEcho(cmd command, ctx shellContext) flowAction {
	fmt.Fprintln(ctx.stdout, strings.Join(cmd.args, " "))
	return actionContinue
}

// builtinExit signals the shell REPL loop to terminate.
// Input: structured command, shell execution context.
// Output: flowAction indicating termination.
func builtinExit(cmd command, ctx shellContext) flowAction {
	return actionExit
}

// builtinType inspects if a target command is a shell builtin or PATH binary.
// Input: structured command, shell execution context.
// Output: flowAction indicating continuation.
func builtinType(cmd command, ctx shellContext) flowAction {
	if len(cmd.args) == 0 {
		return actionContinue
	}

	target := cmd.args[0]
	if ctx.isBuiltin(target) {
		fmt.Fprintf(ctx.stdout, typeBuiltinFmt, target)
		return actionContinue
	}

	if path, ok := findExecutable(target); ok {
		fmt.Fprintf(ctx.stdout, typeExecFmt, target, path)
		return actionContinue
	}

	fmt.Fprintf(ctx.stdout, typeNotFoundFmt, target)
	return actionContinue
}

// runExternal executes external binaries by forwarding configured streams.
// Input: structured command, shell execution context.
// Output: None (runs process to completion).
func runExternal(cmd command, ctx shellContext) {
	if _, ok := findExecutable(cmd.name); !ok {
		fmt.Fprintf(ctx.stdout, cmdNotFoundFmt, cmd.name)
		return
	}

	proc := exec.Command(cmd.name, cmd.args...)
	proc.Stdin = ctx.stdin
	proc.Stdout = ctx.stdout
	proc.Stderr = ctx.stderr

	_ = proc.Run()
}

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
