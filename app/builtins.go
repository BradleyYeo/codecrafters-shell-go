package main

import (
	"fmt"
	"os"
	"strings"
)

// builtinHandler defines the uniform interface for all builtin routines.
type builtinHandler func(cmd command, ctx shellContext) flowAction

func builtinCd(cmd command, ctx shellContext) flowAction {
	if len(cmd.args) == 0{
		return actionContinue
	}
	target := cmd.args[0]
	err := os.Chdir(target)
	if err != nil {
		fmt.Fprintf(ctx.stdout, typeDirNotFound, target);
		return actionContinue
	}

	return actionContinue
}

// builtinPwd prints the current working directory to stdout.
// Input: structured command, shell execution context.
// Output: flowAction indicating continuation.
func builtinPwd(cmd command, ctx shellContext) flowAction {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(ctx.stderr, "Error getting pwd")
		return actionContinue
	}
	fmt.Fprintln(ctx.stdout, pwd)
	return actionContinue
}

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