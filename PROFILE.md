# Micro-Benchmarking & Allocation Profiling

## Writing Benchmarks in [app/main_test.go](file:///Users/bradleyyeo/Documents/learn/go-learn/codecrafters-shell-go/app/main_test.go)
- Create targeted benchmarks for hot paths (`parseCommandLine`, `evalCommand`):

```go
package main

import (
	"io"
	"testing"
)

func BenchmarkParseCommandLine(b *testing.B) {
	line := "echo hello world from benchmark"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseCommandLine(line)
	}
}

func BenchmarkEvalCommandBuiltin(b *testing.B) {
	ctx := shellContext{
		stdin:  nil,
		stdout: io.Discard,
		stderr: io.Discard,
	}
	cmd := command{name: "echo", args: []string{"test", "arg"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = evalCommand(cmd, ctx)
	}
}
```

## Running Benchmarks with Memory Stats
- Run benchmarks measuring operations per second, memory allocations, and heap bytes allocated:

```bash
go test -bench=. -benchmem ./app
```

- Output columns to inspect:
  - `ns/op`: Nanoseconds per iteration (CPU time).
  - `B/op`: Heap bytes allocated per operation.
  - `allocs/op`: Number of distinct heap allocations per operation.

# Profiling CPU and Memory with pprof

## Generating Profiles from Benchmarks
- Run benchmarks with CPU and memory profiling flags to generate profile samples:

```bash
go test -bench=BenchmarkParseCommandLine -cpuprofile=cpu.pprof -memprofile=mem.pprof ./app
```

## Interactive Analysis with pprof CLI
- Inspect the top resource consumers directly in the terminal:

```bash
# Analyze CPU bottlenecks
go tool pprof -top cpu.pprof

# Analyze Memory allocations
go tool pprof -alloc_space -top mem.pprof
```

## Interactive Web UI Analysis
- Launch the browser UI with call graphs, flame graphs, and source-line annotations:

```bash
go tool pprof -http=:8080 cpu.pprof
```

# Compiler Escape Analysis

## Inspecting Stack vs Heap Allocations
- Run Go compiler optimization diagnostics to check where heap allocations occur:

```bash
go build -gcflags="-m -m" ./app
```

## Key Diagnostics to Look For
- `"escapes to heap"`: Variable escapes stack to the heap, triggering Garbage Collector overhead.
- `"can inline"`: Identifies whether small functions (like `findExecutable`) are inlined to remove call overhead.
- `"moved to heap: ..."`: Variables referenced across scopes forced onto the heap.

# Execution Tracing (Syscalls & Latency)

## Capturing Kernel and Runtime Events
- Measure syscall blocking (`sys_read`, `clone`/`execve`) using `runtime/trace`:

```bash
go test -bench=. -trace=trace.out ./app
```

## Visualizing Execution Timeline
- Open the visual trace viewer:

```bash
go tool trace trace.out
```

- Key areas to inspect in the trace:
  - OS Syscall Blocking: Time spent waiting inside `exec.LookPath` or `exec.Cmd.Run()`.
  - Garbage Collection Sweeps: Pauses triggered by allocations in `parseCommandLine`.