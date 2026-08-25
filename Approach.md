# Code Reading Guide for Go Beginners

## Program Entry Point & Execution Trace
- Execution begins at `func main()` in `package main`.
- The REPL creates a `shellContext` with standard streams (`os.Stdin`, `os.Stdout`, `os.Stderr`).
- A `bufio.Reader` reads user input line-by-line in an infinite `for` loop.
- `parseCommandLine` tokenizes the input into a `command` struct containing `name` and `args`.
- `evalCommand` checks the `builtins` map for a matching handler function.
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
- `struct`: Composite data types grouping fields (`shellContext` groups I/O streams; `command` groups `name` and `args`).
- `[]string`: Dynamically sized, contiguous array view (slice) used to store arbitrary argument lists.

### First-Class Functions & Function Types
- `type builtinHandler func(cmd command, ctx shellContext) flowAction`: Treats functions as first-class values.
- Allows storing handler functions directly inside map values for table-driven dispatch.

### Hash Maps and Initialization Lifecycle
- `map[string]builtinHandler`: Key-value lookup table providing O(1) command routing.
- `func init()`: Special runtime lifecycle hook executed before `main()` that safely populates the `builtins` registry without initialization cycles.

### Explicit I/O Writing
- `fmt.Fprint` and `fmt.Fprintf`: Directs formatted output to any target implementing the `io.Writer` interface (`ctx.stdout`) rather than hardcoding global terminal output.

# Architecture

## System Overview

```text
+-------------------------------------------------------------------+
|                            REPL Loop                              |
|   - Holds shellContext (stdin, stdout, stderr)                    |
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
|   - Queries builtins table registry                               |
|   - Routes to builtinHandler or fallback OS driver                |
+---------------------------------+---------------------------------+
                  |                               |
                  v                               v
+---------------------------------+ +-------------------------------+
|     Builtins Table (Registry)   | |      OS Exec Driver           |
|   - map[string]builtinHandler   | |   - exec.LookPath             |
|   - Single Source of Truth      | |   - exec.Command              |
|   - echo, exit, type handlers   | |   - Stream plumbing to ctx    |
+---------------------------------+ +-------------------------------+
```

## Concept Boundaries
- REPL: Controls process lifecycle, prompt display, and unbuffered/buffered stream consumption.
- Context Layer: Injects stream dependencies (`stdin`, `stdout`, `stderr`) eliminating global OS state coupling.
- Parser: Converts unstructured text into intermediate representation (`command` struct).
- Evaluator (Policy): Orchestrates routing decisions without performing execution mechanics.
- Builtin Registry: Unified table mapping command identifiers to handler implementations.
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
- Unifying registration and execution into `map[string]builtinHandler` provides a single authoritative registry for both `type` queries and runtime evaluation.

```go
type builtinHandler func(cmd command, ctx shellContext) flowAction

var builtins map[string]builtinHandler

func init() {
	builtins = map[string]builtinHandler{
		"echo": builtinEcho,
		"exit": builtinExit,
		"type": builtinType,
	}
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

## Dependency Injection & I/O Boundary Isolation
- Hardcoding `os.Stdout`, `os.Stderr`, and `os.Stdin` creates global state coupling and prevents isolated unit testing.
- Encapsulating streams in `shellContext` allows handlers and drivers to operate against abstract `io.Reader` and `io.Writer` interfaces.

```go
type shellContext struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
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

	if handler, ok := builtins[cmd.name]; ok {
		return handler(cmd, ctx)
	}

	runExternal(cmd, ctx)
	return actionContinue
}
```

# Additional Design Principles

## Daniel Jackson Concept Design
- Orthogonal Concepts: Tokenization, routing policy, binary resolution, and process execution are decoupled into independent concepts.
- Intermediate Representation (IR): The `command` struct acts as the single contract between parser and evaluator, preventing parsing details from leaking into execution.

## John Ousterhout Philosophy of Software Design
- Deep Interfaces: `findExecutable` and `parseCommandLine` hide parsing and path traversal complexity behind simple signatures.
- Defining Errors Out of Existence: Empty input returns a zero-value `command{}` that evaluates safely without error handling branches.
- Information Hiding: `evalCommand` coordinates execution without knowing the internal mechanics of `os/exec` or path parsing.

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

## Eliminating Variable and Type Shadowing
- Parameter naming uses distinct identifiers (`name string`, `target string`) rather than reusing the package type identifier (`command`).

```go
func builtinType(cmd command, ctx shellContext) flowAction {
	if len(cmd.args) == 0 {
		return actionContinue
	}

	target := cmd.args[0]
	if _, ok := builtins[target]; ok {
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

// shellContext encapsulates I/O streams to avoid global state dependencies.
type shellContext struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
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
	if _, ok := builtins[target]; ok {
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

// builtins acts as the single source of truth for builtin lookup and execution.
var builtins map[string]builtinHandler

func init() {
	builtins = map[string]builtinHandler{
		"echo": builtinEcho,
		"exit": builtinExit,
		"type": builtinType,
	}
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

	if handler, ok := builtins[cmd.name]; ok {
		return handler(cmd, ctx)
	}

	runExternal(cmd, ctx)
	return actionContinue
}

func main() {
	ctx := shellContext{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	// Initialize reader once on fd 0 to preserve buffered stream state.
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