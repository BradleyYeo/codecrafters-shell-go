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

# Stage 2
+-------------------------------------------------------+
|                     REPL Loop                         |
|   - Prompts user ('$ ')                               |
|   - Reads input stream                                |
+---------------------------+---------------------------+
                            | raw input string
                            v
+-------------------------------------------------------+
|                 Parser / Lexer Layer                  |
|   - Tokenizes args, quotes, pipes, redirections       |
+---------------------------+---------------------------+
                            | command name + []args
                            v
+-------------------------------------------------------+
|                  Execution Engine                     |
|   - Evaluates builtins (exit, echo, type, pwd, cd)    |
|   - Resolves PATH binaries & forks/execs              |
+-------------------------------------------------------+
