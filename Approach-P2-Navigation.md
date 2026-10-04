# Code Reading Guide: Shell Navigation & Modular Architecture

## Execution Flow & Lifecycle
- Execution starts at `func main()` in [app/main.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main.go).
- `shellContext` is initialized with standard I/O streams and the registered builtins table (`echo`, `exit`, `type`, `pwd`, `cd`).
- The REPL prompts `$ `, reads the line with `bufio.Reader`, and tokenizes it via `parseCommandLine`.
- `evalCommand` routes `pwd` to [builtinPwd](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go#L34) and `cd` to [builtinCd](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go#L15) in [app/builtins.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go).
- Unrecognized commands fall back to [runExternal](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/executor.go#L23) in [app/executor.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/executor.go).

# Evolution From Part 1 to Part 2

## Monolithic to Modular Architecture
- Part 1 consolidated the REPL loop, builtin logic, and external process execution inside a single file ([Approach.md](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/Approach.md)).
- Part 2 decomposes the system into three domain modules within `package main`:
  - [app/main.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main.go): Lifecycle, context setup, REPL loop, and dispatch.
  - [app/builtins.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go): In-process command handlers (`echo`, `exit`, `type`, `pwd`, `cd`).
  - [app/executor.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/executor.go): Process execution driver and `$PATH` lookup.

## Stateless to Stateful Execution
- Part 1 handled read-only inspection (`type`, `$PATH` search) and command output (`echo`).
- Part 2 introduces state-mutating commands (`cd`) that alter the running process's working directory state in the Linux kernel.

# Operating System Concepts

## Process Current Working Directory (CWD)
- Every Linux process has a working directory stored in the kernel `task_struct` (`task_struct->fs->pwd`).
- Relative path lookups resolve starting from the directory inode pointed to by the process's CWD.
- Child processes inherit a copy of the parent's CWD upon creation (`fork(2)`).

## The Subprocess Isolation Problem (Why cd Must Be a Builtin)
- An external binary executes in an isolated address space via `fork(2)` and `execve(2)`.
- If `cd` were an external binary (`/bin/cd`), executing it would change the child process's CWD and terminate. The parent shell process's CWD would remain unchanged.
- Commands that mutate shell-level state (working directory, environment variables, shell options) must execute within the shell's own process.

```text
External Binary Execution (Child Process Isolation):
Parent Shell (PID 100, CWD: /app)
    |
    +---> fork() ---> Child Process (PID 101, CWD: /app)
                            |
                            +---> chdir("/tmp") (PID 101 CWD: /tmp)
                            |
                            +---> exit(0) (Child terminates)
    |
Parent Shell (PID 100, CWD: /app)  <-- CWD UNCHANGED!

Builtin Execution (In-Process State Mutation):
Parent Shell (PID 100, CWD: /app)
    |
    +---> builtinCd() ---> os.Chdir("/tmp") (PID 100 CWD: /tmp)
    |
Parent Shell (PID 100, CWD: /tmp)  <-- CWD SUCCESSFULLY MUTATED
```

## System Calls: chdir(2) vs getcwd(2)
- `os.Getwd()`: Issues the `getcwd(2)` system call to retrieve the absolute pathname of the calling process's current working directory.
- `os.Chdir()`: Issues the `chdir(2)` system call to update the calling process's working directory to the target path.

## Path Resolution Mechanics
- Absolute Paths: Paths prefixed with `/` start resolution from the root directory inode.
- Relative Paths: Paths starting with `./`, `../`, or direct directory names resolve relative to the process's current working directory inode. The kernel resolves `.` (current directory) and `..` (parent directory) automatically.
- Tilde Expansion (`~`): `~` is a shell-level syntactic construct, not a filesystem path. The Linux kernel syscall `chdir("~")` fails with `ENOENT` (No such file or directory). The shell must translate `~` to the `$HOME` environment variable before issuing `chdir(2)`.

# Software Design Principles

## Jimmy Koppel Principles

### Reification of Path Normalization
- Decouples path translation from system call invocation.
- Normalizes input arguments upfront (`~` or empty argument -> `$HOME`) before passing to `os.Chdir`.

```text
Input: "cd ~"
   │
   ▼
[ Parse Command ] ──> command{name: "cd", args: ["~"]}
   │
   ▼
[ Path Normalization ] ──> target: "/home/user" (from $HOME)
   │
   ▼
[ Single Syscall ] ──> os.Chdir("/home/user")
   │
   ▼
[ Error Handling ] ──> Success / cdNotFoundFmt
```

### Avoiding Arrow Anti-Pattern
- Uses guard clauses and early returns to maintain flat control flow without nested `if/else` ladders.

```go
func builtinCd(cmd command, ctx shellContext) flowAction {
	target := os.Getenv("HOME")
	if len(cmd.args) > 0 && cmd.args[0] != "~" {
		target = cmd.args[0]
	}

	err := os.Chdir(target)
	if err != nil {
		fmt.Fprintf(ctx.stdout, cdNotFoundFmt, target)
		return actionContinue
	}

	return actionContinue
}
```

## John Ousterhout Principles

### Deep Modules via Cohesion
- [app/builtins.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go) encapsulates all in-process handlers behind the uniform [builtinHandler](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go#L10) interface (`func(cmd command, ctx shellContext) flowAction`).
- [app/executor.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/executor.go) hides external process spawning and stream assignment behind `runExternal`.

### Information Hiding in package main
- Keeping components in `package main` prevents leaking internal functions to outside packages while avoiding premature public API exposure.

## Daniel Jackson Concept Design

### Orthogonal Shell Concepts
- Navigation Concept: State mutator manipulating process working directory (`pwd`, `cd`).
- Lifecycle Concept: REPL loop and flow action dispatch (`flowAction`).
- Parsing Concept: Text to intermediate representation (`command`).
- Execution Concept: Process spawning and stream forwarding (`runExternal`).

# Complete Codebase (Part 2)

## [app/main.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main.go)

```go
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
	cdNotFoundFmt   = "cd: %s: No such file or directory\n"
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
			"cd":   builtinCd,
		},
	}

	// Initialize reader once on fd 0 (Standard Input) to preserve buffered stream state.
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

## [app/builtins.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/builtins.go)

```go
package main

import (
	"fmt"
	"os"
	"strings"
)

// builtinHandler defines the uniform interface for all builtin routines.
type builtinHandler func(cmd command, ctx shellContext) flowAction

// builtinCd changes the current working directory of the shell process.
// Input: structured command, shell execution context.
// Output: flowAction indicating continuation.
func builtinCd(cmd command, ctx shellContext) flowAction {
	target := os.Getenv("HOME")
	if len(cmd.args) > 0 && cmd.args[0] != "~" {
		target = cmd.args[0]
	}

	err := os.Chdir(target)
	if err != nil {
		fmt.Fprintf(ctx.stdout, cdNotFoundFmt, target)
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
```

## [app/executor.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/executor.go)

```go
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
```

# Active Recall & Knowledge Verification

## Questions
- What happens in the Linux kernel when a child process calls `chdir(2)`? Why does this prevent `/bin/cd` from working?
- Why does `os.Chdir("~")` produce `ENOENT` while `cd ~` in Bash works?
- How does `parseCommandLine` handle multiple spaces between arguments?
- What is the difference between `os.UserHomeDir()` and `os.Getenv("HOME")` on Unix systems?
