# Linting

All code must pass `golangci-lint` as configured in `.golangci.yaml`. Run `make lint` before considering any change done. The following rules catch the most common mistakes:

## Wrap all external errors (`wrapcheck`)

Every error returned from a function in another package must be wrapped with `fmt.Errorf("context: %w", err)`. This includes errors from interface methods, not just concrete types:

```go
// Good
if err := xbootstrap.Job(...); err != nil {
    return fmt.Errorf("run job: %w", err)
}

// Bad — linter rejects this
return xbootstrap.Job(...)
```

## Call method values in format strings (`govet`)

When a type has methods, never pass the method as a value; always call it:

```go
// Good
fmt.Sprintf("%s/%s", id.TheaterID(), id.ScreeningID())

// Bad — passes func value, not the string it returns
fmt.Sprintf("%s/%s", id.TheaterID, id.ScreeningID)
```

## Use suite assertions in test suites (`testifylint`)

Inside a `testify/suite` test, use `s.Require()` / `s.Assert()`, never the top-level `require` / `assert` package with `s.T()`:

```go
// Good
s.Require().NoError(err)
s.Require().Equal(expected, actual)

// Bad
require.NoError(s.T(), err)
```

## No `nil, nil` returns (`nilnil`)

Use a sentinel error instead of returning `(nil, nil)` from `(*T, error)` functions.
