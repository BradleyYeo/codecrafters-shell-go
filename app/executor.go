package main

import (
	"fmt"
	"os/exec"
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
