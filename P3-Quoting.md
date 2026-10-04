# Code Reading Guide: Shell Quoting & Lexical State Machines

## Execution Flow & Lifecycle
- Execution begins at `func main()` in [app/main.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main.go).
- `bufio.Reader` captures raw input strings from standard input (`ctx.stdin`).
- `parseCommandLine` delegates lexical analysis to `tokenize` in [app/parser.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/parser.go).
- `tokenize` converts the raw string into a slice of discrete argument tokens (`[]string`), stripping quote delimiters and preserving embedded whitespace.
- `parseCommandLine` maps tokens into a structured `command` struct (`name` and `args`).
- `evalCommand` dispatches to builtins or passes the structured arguments directly to `execve(2)` via `runExternal` in [app/executor.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/executor.go).

# Evolution From Part 2 to Part 3

## Naive Splitting to Lexical Analysis
- Part 2 relied on `strings.Fields(line)` inside `parseCommandLine`.
- `strings.Fields` splits unconditionally on Unicode whitespace and treats quotation marks as regular runes.
- Part 3 introduces a stateful character scanner (Lexer / Tokenizer) capable of context-sensitive delimiter evaluation.

## Semantic Comparison of Inputs
- Input: `echo hello world`
  - Part 2 output: `args = ["hello", "world"]`
  - Part 3 output: `args = ["hello", "world"]`
- Input: `echo 'hello   world'`
  - Part 2 output: `args = ["'hello", "world'"]` (broken: quotes retained, spaces collapsed)
  - Part 3 output: `args = ["hello   world"]` (correct: quotes stripped, inner spaces preserved)
- Input: `echo 'hello''world'`
  - Part 2 output: `args = ["'hello''world'"]` (broken: quotes retained)
  - Part 3 output: `args = ["helloworld"]` (correct: adjacent quoted segments concatenate)
- Input: `cat '/tmp/file name'`
  - Part 2 output: `args = ["'/tmp/file", "name'"]` (broken: split into two invalid file paths)
  - Part 3 output: `args = ["/tmp/file name"]` (correct: single path argument with space preserved)

# Operating System Concepts

## Kernel Process Argument Vector (argv)
- The Linux kernel interface `execve(2)` accepts arguments as an array of null-terminated strings (`char *const argv[]`).
- The kernel does not know what quotes (`'`, `"`) or backslashes (`\`) are.
- In shell syntax, quotes are entirely user-space constructs designed to instruct the shell how to group characters into `argv` entries.
- When executing `cat '/tmp/file name'`, the kernel receives:
  - `argv[0] = "cat"`
  - `argv[1] = "/tmp/file name"`
- If the shell passes quotes to the binary (`argv[1] = "'/tmp/file name'"`), `cat` requests an `open(2)` syscall for a file literally containing single quotes in its filename, failing with `ENOENT`.

```text
Shell Input (User Space):
    cat '/tmp/file name'
            │
            ▼ [ Lexer / Tokenizer ]
Tokens:
    ["cat", "/tmp/file name"]  <-- Quotes stripped, space preserved
            │
            ▼ [ execve(2) Syscall ]
Kernel Space (argv array):
    argv[0] ──> "cat\0"
    argv[1] ──> "/tmp/file name\0"
    argv[2] ──> NULL
```

## POSIX Shell Quote Removal
- In the POSIX standard (IEEE Std 1003.1-2017, Section 2.6.7 Quote Removal), after the shell finishes word expansion and field splitting, all unquoted quotation marks (`'`, `"`) and unquoted backslashes (`\`) are discarded.
- Single quotes treat every enclosed character literally. No escape sequences exist within single quotes (e.g. `'\\'` produces `\\`).

# Software Design Principles

## Daniel Jackson Concept Design

### The Tokenizer Concept vs Command Concept
- Tokenizer Concept: Transforms a stream of input runes into a sequence of semantic words (`[]string`), resolving quoting contexts.
- Command Concept: Takes a sequence of semantic words, interprets `argv[0]` as an executable/builtin target, and `argv[1:]` as parameters.
- Orthogonality: Quoting rules belong strictly to the Tokenizer concept. Builtin handlers (`builtinEcho`, `builtinCd`) and external execution drivers (`runExternal`) must never inspect or strip quotes.

## Jimmy Koppel Principles

### Reification of Lexer State
- Avoid tracking parsing state using an ad-hoc combination of boolean flags (`inQuote`, `escaped`, `isDouble`).
- Reify parser states into an explicit enumeration type (`parseState`).
- The lexer becomes a deterministic finite state machine (FSM).

### Reification of Token Emission
- The logic to flush an accumulated token exists at two distinct points: upon encountering unquoted whitespace and at end-of-line (EOF).
- Duplicating the flush logic causes state drift and increases the defect surface as more states are added.
- Reify token flushing into a single localized closure (`flush`) that handles token emission, buffer clearing, and state reset atomically.

### Avoiding Arrow Anti-Pattern
- Avoid deeply nested `switch` inside `case` inside `switch` ladders.
- Flatten character handling using top-level state partitioning with early `continue` guard clauses.

```text
Arrow Anti-Pattern (Nested, Obscure):
for _, r := range line {
    switch state {
    case stateNormal:
        switch {
        case r == '\'':
            state = stateInSingleQuote
            inToken = true
        case unicode.IsSpace(r):
            ...
        }
    }
}

Flattened State Dispatch (Linear, Flat):
for _, r := range line {
    switch state {
    case stateInSingleQuote:
        if r == singleQuote {
            state = stateNormal
            continue
        }
        current.WriteRune(r)

    case stateNormal:
        if r == singleQuote {
            state = stateInSingleQuote
            inToken = true
            continue
        }
        if unicode.IsSpace(r) {
            flush()
            continue
        }
        current.WriteRune(r)
        inToken = true
    }
}
```

### Abstraction Boundary Hierarchy
- Strict communication rule: Each layer communicates only with its immediate neighbor.
- Layer 0 (Input Stream): `bufio.Reader` reads raw text lines.
- Layer 1 (Lexer): `tokenize(line string) []string` scans runes and yields clean tokens.
- Layer 2 (Syntax Builder): `parseCommandLine(line string) command` groups tokens into `command`.
- Layer 3 (Policy / Dispatch): `evalCommand(cmd command, ctx shellContext) flowAction` routes execution.
- Layer 4 (Execution Mechanics): `builtinHandler` or `runExternal` invokes system operations.

## John Ousterhout Principles

### Deep Modules
- Interface: `tokenize(line string) []string`.
- The interface is minimal (one string input, one token slice output).
- The implementation is deep: it encapsulates state transitions, rune iteration, whitespace boundary detection, and token accumulation without exposing internal buffers.

### Information Hiding
- Lexer states, rune buffers, and scanning state variables are private to [app/parser.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/parser.go).
- Calling code in [app/main.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main.go) receives already-delimited strings.

### Defining Errors Out of Existence
- What happens if a command line ends with an unclosed single quote (e.g. `echo 'unterminated`)?
- Instead of throwing fatal panics or crashing the shell process, the lexer flushes the current buffer as the final argument and terminates cleanly.

### Reducing Cognitive Clutter
- Avoid speculative constants: Declaring `doubleQuote`, `backslash`, `spaceChar`, and `tabChar` before they are consumed increases cognitive load.
- Ensure declared constants are actively consumed rather than bypassed with hardcoded character literals (`singleQuote` vs `'\''`).

# Design Tradeoffs

## Closure vs Standalone Function for Token Flushing
- Option A: Private package function `flushToken(tokens *[]string, current *strings.Builder, inToken *bool)`.
  - Tradeoff: Creates a shallow function with 3 pointer arguments, polluting package scope with lexer plumbing.
- Option B: Scoped closure `flush := func() { ... }` inside `tokenize`.
  - Tradeoff: Captures lexical variables directly by reference, keeps emission logic cohesive, and prevents accidental external state mutations.

## Switch-Based FSM vs Object-Oriented State Pattern
- Option A: State interface (`type State interface { handle(r rune) State }`) with distinct structs per state.
  - Tradeoff: Introduces heap allocations and dynamic dispatch overhead for every character processed.
- Option B: Value-based enum (`parseState`) inside a `switch`.
  - Tradeoff: Zero allocations per character, optimal CPU cache locality, and clear visibility of all transitions in a single code block.

# Extensible Design for Subsequent Stages

## Anticipated Requirements
- Stage 1: Single quotes (`'...'`) — All characters literal, spaces preserved.
- Stage 2: Double quotes (`"..."`) — Spaces preserved, supports escaping and parameter interpolation.
- Stage 3: Backslash outside quotes (`\`) — Escapes following character (e.g. `\ ` -> literal space).
- Stage 4: Backslash within single quotes — Preserved literally (e.g. `'\n'` -> `\n`).
- Stage 5: Backslash within double quotes — Escapes `\$`, `\"`, `\\`, and `\n`.
- Stage 6: Quoted executable — First token (`argv[0]`) stripped of quotes before PATH resolution.

## Extensible State Machine Transition Diagram

```text
                         +-------------------+
                         |                   |
                         |    stateNormal    |<--------------------+
                         |                   |                     |
                         +-------------------+                     |
                           /        |        \                     |
             Single Quote /         | Double  \ Escape             |
                   ('\'')/          | Quote    \ ('\\')            |
                        /           | ('"')     \                  |
                       v            v            v                 |
       +--------------------+  +--------------------+  +-------------------+
       | stateInSingleQuote |  | stateInDoubleQuote |  |    stateEscape    |
       |  (All literal)     |  | (Future: expand,   |  | (Future: literal  |
       |                    |  |  allow escapes)    |  |  next char)       |
       +--------------------+  +--------------------+  +-------------------+
               |                         |                       |
               +-------------------------+-----------------------+
                              Return to stateNormal
```

## State Enumeration & Active Constants
- Maintain only active states and constants to prevent dead-code friction.

```go
type parseState int

const (
	stateNormal parseState = iota
	stateInSingleQuote
	// Extensible slots for future stages:
	// stateInDoubleQuote
	// stateEscape
)

const singleQuote = '\''
```

## Token Concatenation Mechanics
- In shell parsing, a token is delimited only by unquoted whitespace.
- Quoted strings and unquoted strings placed directly adjacent to each other belong to the same token:
  - Input: `'hello''world'` -> Token: `helloworld`
  - Input: `hello''world` -> Token: `helloworld`
  - Input: `''` -> Token: `""` (empty string argument)
- State tracking requires:
  - `strings.Builder current`: Accumulates runes belonging to the current word.
  - `bool inToken`: Tracks whether a token is actively being constructed. This distinction is critical: an empty quoted string `''` produces an argument with length 0, whereas whitespace produces no argument.

```text
Tracing `echo hello''world`:
1. 'h', 'e', 'l', 'l', 'o' -> current = "hello", inToken = true
2. singleQuote in stateNormal -> state = stateInSingleQuote, inToken = true
3. singleQuote in stateInSingleQuote -> state = stateNormal, inToken = true
4. 'w', 'o', 'r', 'l', 'd' -> current = "helloworld", inToken = true
5. EOF -> flush() -> tokens = ["echo", "helloworld"]
```

# Implementation Specification

## Module: [app/parser.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/parser.go)

```go
package main

import (
	"strings"
	"unicode"
)

// parseState defines lexical scanner modes.
type parseState int

const (
	stateNormal parseState = iota
	stateInSingleQuote
)

const singleQuote = '\''

// tokenize parses a raw command line into semantic argument tokens.
// Input: raw command string from standard input (e.g., "echo 'foo bar'").
// Output: slice of parsed arguments with quotes stripped and spaces preserved.
func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder

	state := stateNormal
	inToken := false

	flush := func() {
		if inToken {
			tokens = append(tokens, current.String())
			current.Reset()
			inToken = false
		}
	}

	for _, r := range line {
		switch state {
		case stateInSingleQuote:
			if r == singleQuote {
				state = stateNormal
				continue
			}

			current.WriteRune(r)

		case stateNormal:
			if r == singleQuote {
				state = stateInSingleQuote
				inToken = true
				continue
			}

			if unicode.IsSpace(r) {
				flush()
				continue
			}

			current.WriteRune(r)
			inToken = true
		}
	}

	flush()

	return tokens
}
```

## Module: [app/main.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main.go) Integration

```go
// parseCommandLine tokenizes a raw input line into a structured command.
// Input: raw string from stdin (e.g., "echo 'foo bar'").
// Output: command struct containing the binary/builtin name and arguments.
func parseCommandLine(line string) command {
	tokens := tokenize(line)
	if len(tokens) == 0 {
		return command{}
	}

	return command{
		name: tokens[0],
		args: tokens[1:],
	}
}
```

# Active Recall & Knowledge Verification

## Questions
- Why does `execve(2)` fail when a shell passes raw quotes (e.g., `argv[1] = "'file name'"`), but succeeds with `argv[1] = "file name"`?
- What distinguishes an empty string argument (`echo ''`) from whitespace between arguments (`echo   `) at the lexer level?
- How does flattening the nested `switch` with early `continue` statements reduce cognitive load and prevent the Arrow Anti-Pattern?
- Why is an inline closure (`flush`) preferred over a package-level helper function for token emission?
- What performance and architectural advantages does an enum-based `switch` state machine have over an object-oriented state pattern in Go?
- Why must single-quote stripping occur inside the Tokenizer concept rather than inside `builtinEcho` or `runExternal`?
