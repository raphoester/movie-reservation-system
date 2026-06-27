# Tooling Conventions

Conventions for CLI programs under `internal/tooling/cmd/`. These apply to local developer tools, not to services.

## Structure

Every CLI tool must decompose its `main` into named single-purpose stages. The top-level `run` function should read like a table of contents — it names the stages and wires them together, nothing more.

```go
// Good
func run(ctx context.Context, logger *slog.Logger) error {
    flags, err := parseFlags()
    if err != nil {
        return err
    }
    results, err := doWork(ctx, flags, ...)
    if err != nil {
        return err
    }
    ok := displayResults(results)
    if !ok {
        return fmt.Errorf("one or more checks failed")
    }
    return nil
}

// Bad — monolithic run that parses, executes, and prints inline
func run(logger *slog.Logger) error {
    servicePath := flag.String(...)
    flag.Parse()
    // ... business logic ...
    // ... printing ...
    anyFailed := false
    // ...
}
```

**Rules:**

- **`parseFlags`** returns a typed config struct (e.g. `FlagsConfig`). Never pass raw `*string` flag values as arguments to downstream functions.
- **Work functions** return values — they do not side-effect into outer-scope variables. A goroutine body should ideally be a single assignment: `results[i] = doOneUnit(...)`.
- **Display functions** return a `bool` (or status). They own output, not error propagation. The caller decides what to do with the result.
- **Function signatures are honest**: if a function can fail, it returns an `error`; if it can't, it doesn't.
