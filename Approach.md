# Code Reading Guide for Go Beginners

## Program Entry Point & Execution Trace
- Execution begins at `func main()` in `package main`.
- The REPL initializes a `shellContext` holding standard streams (`os.Stdin`, `os.Stdout`, `os.Stderr`) and the `builtins` dispatch table.
- A `bufio.Reader` reads user input line-by-line in an infinite `for` loop.
- `parseCommandLine` tokenizes the input into a `command` struct containing `name` and `args`.
- `evalCommand` checks `ctx.builtins` for a matching handler function.
- If found, it executes the builtin handler (`builtinEcho`, `builtinExit`, `builtinType`).
- If not found, `runExternal` searches `$PATH` via `findExecutable` and launches a child process via `os/exec`.
- The returned `flowAction` determines whether the loop continues or breaks on `actionExit`.

## Go Language Constructs Explained

### Packages and Imports
- `package main`: Identifies this file as a standalone executable program rather than a library.
- `import (...)`: Standard library modules used for I/O (`io`, `bufio`, `fmt`, `os`), process execution (`os/exec`), and string manipulation (`strings`).

### Constants and Iota Enums
- `const (...)`: Immutable values evaluated at compile time to avoid repeated string allocation.
- `type flowAction int` + `iota`: Go's idiomatic pattern for auto-incrementing integer enumerations (`actionContinue = 0`, `actionExit = 1`).

### Structs and Slices
- `struct`: Composite data types grouping fields (`command` groups `name` and `args`; `shellContext` groups I/O streams and the registry).
- `[]string`: Dynamically sized, contiguous array view (slice) used to store arbitrary argument lists.

### First-Class Functions & Function Types
- `type builtinHandler func(cmd command, ctx shellContext) flowAction`: Treats functions as first-class values.
- Allows storing handler functions directly inside map values for table-driven dispatch.

### Methods on Structs
- `func (ctx shellContext) isBuiltin(name string) bool`: A method with a value receiver attached to `shellContext`, encapsulating membership checks.

### Explicit I/O Writing
- `fmt.Fprint` and `fmt.Fprintf`: Directs formatted output to any target implementing the `io.Writer` interface (`ctx.stdout`) rather than hardcoding global terminal output.

# Architecture

## System Overview

```text
+-------------------------------------------------------------------+
|                            REPL Loop                              |
|   - Holds shellContext (stdin, stdout, stderr, builtins)          |
|   - Prompts user ('$ ') to stdout                                 |
|   - Reads input stream from stdin (bufio.Reader)                  |
+---------------------------------+---------------------------------+
                                  | raw input string
                                  v
+-------------------------------------------------------------------+
|                       Parser / Tokenizer                          |
|   - Converts raw input into structured command (IR)               |
+---------------------------------+---------------------------------+
                                  | command{name, args}
                                  v
+-------------------------------------------------------------------+
|                    Command Evaluator (Policy)                     |
|   - Queries ctx.builtins registry                                 |
|   - Routes to builtinHandler or fallback OS driver                |
+---------------------------------+---------------------------------+
                  |                               |
                  v                               v
+---------------------------------+ +-------------------------------+
|     Builtins Table (Registry)   | |      OS Exec Driver           |
|   - ctx.builtins map            | |   - exec.LookPath             |
|   - ctx.isBuiltin() helper      | |   - exec.Command              |
|   - echo, exit, type handlers   | |   - Stream plumbing to ctx    |
+---------------------------------+ +-------------------------------+
```

## Concept Boundaries
- REPL: Controls process lifecycle, prompt display, and unbuffered/buffered stream consumption.
- Context Layer: Injects stream dependencies (`stdin`, `stdout`, `stderr`) and the dispatch table (`builtins`), eliminating global state.
- Parser: Converts unstructured text into intermediate representation (`command` struct).
- Evaluator (Policy): Orchestrates routing decisions without performing execution mechanics.
- Builtin Registry: Injected table mapping command identifiers to handler implementations.
- OS Exec Driver (Mechanism): Interfaces with Linux kernel syscalls (`fork`/`execve`, stream binding).

# Operating System Concepts

## Process Execution Lifecycle
- External commands require creating child processes via kernel primitives (`fork`/`execve` model).
- Standard streams (File Descriptors 0, 1, 2) must be forwarded from parent shell to child process.
- Stream plumbing binds child `Stdin`, `Stdout`, and `Stderr` directly to `ctx.stdin`, `ctx.stdout`, and `ctx.stderr`.

## Binary Resolution via PATH
- `$PATH` environment variable contains colon-delimited directory paths.
- Resolving executables requires sequential directory scans with permission checks for executable bits (`mode & 0111`).
- `exec.LookPath` abstracts the file system traversal and permission verification.

## Shell Builtins vs External Binaries
- Builtins run in-process: they can mutate shell environment state (such as working directory, exit status, variables).
- External binaries execute in isolated memory spaces: child process mutations do not affect the parent shell.
- `type` queries the resolution hierarchy: Builtins take precedence, followed by `$PATH` filesystem binaries.

## Stream Buffering Mechanics
- `bufio.Reader` wraps standard input to avoid per-byte syscall overhead.
- Reader must be instantiated once at shell initialization to prevent dropped bytes across read cycles.

# Jimmy Koppel Software Design Principles

## Single Source of Truth & Table-Driven Dispatch
- Duplicated registrations cause Shotgun Surgery: adding a builtin previously required changing both a lookup set and a dispatch switch block.
- Injected `builtins` registry provides a single authoritative mapping for both `type` queries (`ctx.isBuiltin`) and runtime evaluation (`ctx.builtins[cmd.name]`).

```go
type builtinHandler func(cmd command, ctx shellContext) flowAction

type shellContext struct {
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	builtins map[string]builtinHandler
}

func (ctx shellContext) isBuiltin(name string) bool {
	_, ok := ctx.builtins[name]
	return ok
}
```

## Reification of Control Flow
- Returning boolean flags (`bool`) creates ambiguous control flows and primitive obsession.
- Modeling state transitions as an explicit `flowAction` enum makes loop termination and continuation first-class concepts.

```go
type flowAction int

const (
	actionContinue flowAction = iota
	actionExit
)
```

## Dependency Injection & Zero Global State
- Hardcoding `os.Stdout`, `os.Stderr`, and package-level `var builtins` creates global state coupling and circular initialization dependencies.
- Injecting all dependencies through `shellContext` enables headless testing and deterministic execution.

```go
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
    // ...
}
```

## Separation of Policy from Mechanism
- Evaluator contains pure policy (determining whether a command is builtin or external).
- Builtin handlers and OS driver encapsulate mechanisms (string joining, exit signalling, stream forwarding).

```go
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
```

# John Ousterhout Software Design Principles

## Information Hiding vs Information Leakage
- `builtinType` queries `ctx.isBuiltin(target)` without knowing whether the registry is stored in a map, slice, or database.
- Eliminates circular package initialization dependencies by eliminating package-level global registries.

```go
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
```

## Deep Interfaces and Encapsulation
- `findExecutable`: Compact signature `(string) -> (string, bool)` completely hides `$PATH` parsing and permissions checks.
- `runExternal`: Pulls down child process spawning, stream assignment, and execution error handling.

## Defining Errors Out of Existence
- `parseCommandLine`: Blank inputs produce a zero-value `command{}` rather than returning errors.
- `evalCommand`: Handles `cmd.name == ""` by no-oping with `actionContinue`, removing error-handling branches from the REPL loop.
- `builtinType`: Returns `actionContinue` on empty arguments instead of panicking on index out of range.

# Daniel Jackson Concept Design

## Orthogonal Concepts
- Tokenization: Pure data transformation (`string -> command{name, args}`).
- Routing Policy: Evaluates token identifiers against registered capabilities (`evalCommand`).
- Binary Resolution: Independent filesystem query on `$PATH` (`findExecutable`).
- Process Execution: Low-level OS execution driver (`runExternal`).

## Intermediate Representation (IR)
- The `command` struct acts as the single contract between parser and evaluator, preventing parsing details from leaking into execution.

# Idiomatic Go Patterns & Code Examples

## Zero-Value Struct Safety
- Returning `command` by value instead of `*command` eliminates pointer indirection and nil-pointer exceptions.
- Empty input strings naturally produce a valid zero-value struct (`command{name: "", args: nil}`).

```go
type command struct {
	name string
	args []string
}

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
```

## Guard Clauses and Early Returns
- Validates failure conditions upfront and returns immediately to keep the main execution path at the lowest indentation level.

```go
func findExecutable(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}

	return path, true
}
```

## Standard Stream Forwarding with os/exec
- Explicit binding of child process descriptors to parent standard streams via `*exec.Cmd`.

```go
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
```

# Knowledge Gaps & Next Steps

## Concepts to Master Next
- Lexing with Quotes: Handling single quotes (literal strings) and double quotes (variable expansion / escape sequences).
- Shell State Mutators: Implementing `cd` and `pwd` using `os.Chdir` and `os.Getwd`.
- POSIX Pipelines: Inter-process communication via `os.Pipe` connecting stdout of process `N` to stdin of process `N+1`.
- Stream Redirection: Intercepting FD 1 (`>`) and FD 2 (`2>`) to write to file descriptors instead of standard terminal streams.

## Active Recall Questions
- Why must `cd` be implemented as a shell builtin rather than an external binary?
- What happens if `bufio.NewReader(os.Stdin)` is instantiated inside the loop instead of before the loop?
- How does `exec.LookPath` determine if a file is executable on Unix systems?
- What is the difference between `exec.Cmd.Run()` and `exec.Cmd.Start()`?

# Full Solution

```go
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
```