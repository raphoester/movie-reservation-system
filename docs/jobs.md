# Jobs

A job is a one-shot background process (bootstrapped via `xbootstrap.Job`). It must respect the same clean-architecture rules as any other service.

## Jobs are side-effect-free

The core logic — data fetching, computation, diffing — must not log, print, or write anywhere. It collects its findings into a result struct and returns it. `main.go` owns all side effects: it calls the job, then logs the result.

## Logs belong in decorators, not in domain objects

If per-item progress is useful, wrap the processor in a logging decorator rather than adding `logger.Info` calls inside the core type:

```go
// Core processor — pure, no logging
type HTTPScreeningProcessor struct { ... }
func (p *HTTPScreeningProcessor) Process(ctx, screeningID, knownSet) ScreeningResult { ... }

// Logging decorator — wraps core, adds observability
type LoggingScreeningProcessor struct {
    inner  ScreeningProcessor
    logger *slog.Logger
}
func (p *LoggingScreeningProcessor) Process(ctx, screeningID, knownSet) ScreeningResult {
    result := p.inner.Process(ctx, screeningID, knownSet)
    p.logger.Info("screening probe done", "screening_id", screeningID, "missing", len(result.MissingIDs))
    return result
}
```

## Collect, then report

The job's `Run` method returns a result struct. `main.go` logs the full summary once at the end:

```go
result, err := discoverer.Run(ctx)
if err != nil { return fmt.Errorf("run discovery: %w", err) }
logger.Info("discovery complete", "total_missing", len(result.MissingIDs))
for _, id := range result.MissingIDs {
    logger.Info("missing screening", "source_screening_id", id)
}
```

Never scatter `fmt.Printf` or `logger.Info` calls throughout the domain logic. The domain returns data; the entry point decides what to do with it.
