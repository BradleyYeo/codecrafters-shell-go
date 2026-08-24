# Stage 1
+------------------------------------------+
|  REPL (Prompt & Line Reader)             |
+------------------------------------------+
                    |
                    v
+------------------------------------------+
|  Command Dispatcher / Evaluator          |
+------------------------------------------+
        |                          |
        v                          v
+------------------+     +-----------------+
| Builtin Registry |     | OS Exec Driver  |
+------------------+     +-----------------+

Separate UI/IO from Command Evaluation
Concept Boundaries:
Prompt / REPL: Manages shell prompt and I/O streams.
Parser: Converts raw stdin bytes into command tokens.
Evaluator / Dispatcher: Resolves builtins or external binaries and executes them.