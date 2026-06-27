# Coding Conventions

General coding conventions for Go services in this codebase. These apply to any service or package regardless of domain.

## Object-Oriented Design

HTTP handlers must be thin. They parse input, delegate to domain objects, and map errors to HTTP responses. They must never contain business logic, transaction management, or serialization inline.

Business logic belongs in purpose-built domain objects. Every meaningful operation — a validation rule, a transactional write, an event — is its own type with a constructor and a method. Objects do things; they are not done to (Tell, Don't Ask principle, Demeter's Law).

**Rule objects** enforce a single invariant and return a sentinel error:

```go
// Good
type SingleProviderCheck struct { ... }
func (r SingleProviderCheck) Check(ctx context.Context) error { ... }

// Bad — inline guard in the handler
if provider.IsDynamic() {
    existing, _ := storage.GetByID(ctx, id)
    for _, b := range existing.Items { ... }
}
```

**Command objects** own their full lifecycle (transaction, persistence, event):

```go
// Good
booking.NewCreate(uow, params, event).Execute(ctx)

// Bad — tx.Begin / Insert / Publish / Commit scattered in the handler
```

**Event objects** own their own serialization:

```go
// Good
msg, err := booking.NewCreatedEvent(...).AsMessage()

// Bad — json.Marshal + CreateMessage called inline in the handler
```

**Value constructors** replace anemic struct literals:

```go
// Good
booking.NewProviderReference(provider, providerID)

// Bad
booking.ProviderReference{Provider: provider, ProviderID: providerID}
```

Reference: _Elegant Objects_ by Yegor Bugayenko.

## Immutability

Domain objects must be immutable. Never mutate an object in place — this leads to unexpected behavior and makes reasoning about state harder. To modify an object before persisting it, use builder methods that return a new copy with the updated field:

```go
// Good — returns a new copy
func (s Screening) WithStartTime(startTime time.Time) Screening {
    s.startTime = startTime
    return s
}

// Bad — mutates in place
func (s *Screening) SetStartTime(startTime time.Time) {
    s.startTime = startTime
}
```

This applies to all domain structs. Exceptions are rare and must be justified.

## Snapshot Pattern

Private fields for invariant safety, `TakeSnapshot()` for persistence/serialization, `Restore()` as the single validation point on both read and write paths.

→ [docs/snapshot-pattern.md](snapshot-pattern.md)

## Slice Mapping

When mapping one slice to another, use `append` with initial capacity allocation rather than index assignment:

```go
// Good — append with capacity
items := make([]Item, 0, len(source))
for _, s := range source {
    items = append(items, mapToItem(s))
}

// Bad — index assignment
items := make([]Item, len(source))
for i, s := range source {
    items[i] = mapToItem(s)
}
```

## Sets

Prefer a dedicated Set type over `map[T]struct{}` for set semantics. This codebase provides `xcolls.Set` (`internal/shared/xcolls/`) — use `xcolls.NewSet(items...)` or `xcolls.NewSetWithSize[T](n)`.

## Comments

Never write placeholder or obvious comments. In particular, never use comments like `// handle error`, `// TODO: implement`, or `// call the function` — write the actual code instead, or write nothing. The only comments that belong in code are those that explain _why_ something non-obvious is done, not _what_ it does.

## Command and Query Naming

Commands and queries are named without a `Command`/`Query` suffix (e.g. `CreateBooking`, not `CreateBookingCommand`). The package qualifier already carries that context at the call site:

```go
command.CreateBooking        // the input struct
command.CreateBookingResult  // the output struct
*command.CreateBookingHandler // the handler
```

The handler is a separate type from the command struct, so there is no ambiguity. Adding a suffix would be redundant noise — `command.CreateBookingCommand` says "command" twice.

## Sentinel Errors over nil, nil

Never return `(nil, nil)` from a function that returns `(*T, error)`. A `nil` result that is not an error must be represented by a dedicated sentinel error so callers can distinguish "not found / absent" from a real failure with a single `errors.Is` check:

```go
// Good
var ErrSeatNotInScreening = errors.New("seat not in screening")

func (s *Storage) GetByScreeningID(ctx context.Context, id string) (*Seat, error) {
    // ... query ...
    if notFound {
        return nil, ErrSeatNotInScreening
    }
    return seat, nil
}

// Caller
seat, err := storage.GetByScreeningID(ctx, id)
if err != nil && !errors.Is(err, seating.ErrSeatNotInScreening) {
    return fmt.Errorf("unexpected error: %w", err)
}
// seat is nil when err == ErrSeatNotInScreening

// Bad
if notFound {
    return nil, nil  // caller cannot tell "not found" from success
}
```

This rule is enforced by the `nilnil` linter.

## Enum Conversions

At every layer boundary (DB→domain, domain→HTTP) enum string values must be converted through a named function that returns `error` for unrecognized inputs. Direct type casts (`EnumType(someString)`) across layers are forbidden because they silently accept garbage values.

The pattern mirrors `StarsFromInt`:

```go
func StatusFromString(s string) (Status, error) {
    switch Status(s) {
    case StatusActive, StatusDisabled:
        return Status(s), nil
    default:
        return "", fmt.Errorf("unknown screening status: %q", s)
    }
}
```

For enums with many values, use a set for validation:

```go
var knownProviders = xcolls.NewSet(
    ProviderStripe, ProviderPayPal, ProviderManual,
)

func ProviderFromString(s string) (Provider, error) {
    normalized := Provider(strings.ToLower(s))
    if !knownProviders.Contains(normalized) {
        return "", fmt.Errorf("unknown provider: %q", s)
    }
    return normalized, nil
}
```

The only acceptable plain cast across layers is after explicit validation within the conversion function itself (e.g., `return Provider(strings.ToLower(s))` at the end of `ProviderFromString`, not at the call site).

## Jobs

Side-effect-free core logic, logs in decorators, collect-then-report in `main.go`.

→ [docs/jobs.md](jobs.md)

## Linting

All code must pass `golangci-lint` (`make lint`). Common rules: wrap external errors with `fmt.Errorf`, always call method values in format strings, use `s.Require()` not `require.NoError(s.T(), ...)` in suites, no `nil, nil` returns.

→ [docs/linting.md](linting.md)

## Testing

→ [docs/testing.md](testing.md)
