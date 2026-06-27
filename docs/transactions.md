# Transactions & Outbox Pattern

This document describes the recipe for handling database transactions and transactional event publishing. The ground transportation booking service is a complete reference implementation; the concepts apply to any service regardless of package layout.

## Overview

The pattern centres on four port interfaces defined per service:

| Interface | Role |
|---|---|
| `UnitOfWork` | Factory — begins a transaction |
| `Transaction` | Scopes a single database transaction; provides repositories and event publisher |
| `TransactionalRepositories` | Groups all repositories bound to the active transaction |
| `EventPublisher` | Publishes events into the outbox within the same transaction |

Because these are interfaces, the infrastructure layer can provide a real PostgreSQL implementation for production, and an in-memory implementation for unit tests — with no changes to application code.

## The Interfaces

These interfaces are defined per service. `TransactionalRepositories` and `EventPublisher` are intentionally service-specific because each service has a different set of repositories, and may have other adapters that need to participate in the same transactional unit.

```go
type UnitOfWork interface {
    Begin(ctx context.Context) (Transaction, error)
}

type Transaction interface {
    Commit() error
    Rollback() error
    Repositories() TransactionalRepositories
    EventPublisher() EventPublisher
}

// TransactionalRepositories lists every repository that must participate in the transaction.
// Add or remove accessors to match the service's domain.
type TransactionalRepositories interface {
    Bookings() BookingRepository
    // ...
}

type EventPublisher interface {
    Publish(topic string, msg *message.Message) error
}
```

## Repository Design: Accept `xpg.Querier`

Repository implementations must accept `xpg.Querier`, not `*sql.DB` or `*sql.Tx` directly:

```go
// internal/shared/xpg/postgres.go
type Querier interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

Both `*sql.DB` and `*sql.Tx` satisfy this interface. A single constructor handles both cases:

```go
type BookingRepository struct {
    db xpg.Querier
}

func NewBookingRepository(db xpg.Querier) *BookingRepository {
    return &BookingRepository{db: db}
}
```

Pass the bare `*sql.DB` (or `*xpg.Postgres`) for read-side queries. Pass `*sql.Tx` inside the `UnitOfWork` implementation when constructing the transactional repositories.

## Usage in Command Handlers

Handlers receive `UnitOfWork` as a dependency and follow this pattern:

```go
tx, err := h.unitOfWork.Begin(ctx)
if err != nil {
    return fmt.Errorf("begin transaction: %w", err)
}
defer func() { _ = tx.Rollback() }()

if err := tx.Repositories().Bookings().Create(ctx, booking); err != nil {
    return fmt.Errorf("persist booking: %w", err)
}

msg, err := xwatermill.CreateMessage(events.BookingCreated{BookingID: booking.ID().String()})
if err != nil {
    return fmt.Errorf("create event: %w", err)
}
if err := tx.EventPublisher().Publish(msg.Topic, msg.Msg); err != nil {
    return fmt.Errorf("publish event: %w", err)
}

if err := tx.Commit(); err != nil {
    return fmt.Errorf("commit: %w", err)
}
```

Key points:
- `defer tx.Rollback()` is always safe; after a successful `Commit()`, PostgreSQL ignores a subsequent rollback.
- Both the repository write and the outbox insert happen within the same `*sql.Tx`, so they are committed or rolled back atomically.

## Database Implementation

The concrete implementation holds an `xpg.Beginner` (satisfied by `*xpg.Postgres`) and begins a `*sql.Tx` on each `Begin` call:

```go
type DatabaseUnitOfWork struct {
    beginner xpg.Beginner
    logger   watermill.LoggerAdapter
}

func (u *DatabaseUnitOfWork) Begin(ctx context.Context) (Transaction, error) {
    tx, err := u.beginner.BeginTx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("begin transaction: %w", err)
    }
    return newDatabaseTransaction(tx, u.logger)
}
```

`DatabaseTransaction` wires the `*sql.Tx` into repositories and into the outbox publisher:

```go
type DatabaseTransaction struct {
    tx           *sql.Tx
    repositories *transactionalRepositories
    publisher    *routingPublisher
}

func newDatabaseTransaction(tx *sql.Tx, logger watermill.LoggerAdapter) (*DatabaseTransaction, error) {
    sqlPublisher, err := xwatermill.NewOutboxPublisher(tx, logger, xwatermill.WithSchema("my_service"))
    if err != nil {
        return nil, fmt.Errorf("create outbox publisher: %w", err)
    }

    return &DatabaseTransaction{
        tx:           tx,
        repositories: newTransactionalRepositories(tx),
        publisher:    sqlPublisher,
    }, nil
}
```

If the service also needs delayed events, the publisher can route based on message metadata — see the booking service's `routingPublisher` for the pattern.

The transactional repositories are instantiated by passing `tx` directly to each repository constructor (which accepts `xpg.Querier`):

```go
func newTransactionalRepositories(tx *sql.Tx) *transactionalRepositories {
    return &transactionalRepositories{
        bookings: NewBookingRepository(tx),
        // ...
    }
}
```

## In-Memory Implementation (Testing)

An in-memory implementation satisfies the same interfaces with no database dependency. `Commit` and `Rollback` are no-ops; the same repository instances are shared across `Begin` calls so data seeded before the transaction is visible inside it:

```go
type InMemUnitOfWork struct {
    bookingRepo *InMemBookingRepository
    publisher   EventPublisher
}

func (u *InMemUnitOfWork) Begin(_ context.Context) (Transaction, error) {
    return &inMemTransaction{
        bookingRepo: u.bookingRepo,
        publisher:   u.publisher,
    }, nil
}

func (t *inMemTransaction) Commit() error   { return nil }
func (t *inMemTransaction) Rollback() error { return nil }
```

Use this in unit tests to assert on stored state or published events without spinning up a database.

## Checklist for a New Service

1. Define `UnitOfWork`, `Transaction`, `TransactionalRepositories`, and `EventPublisher` interfaces — keep them in a location that your command handlers can import.
2. Implement repositories accepting `xpg.Querier` with a single constructor.
3. Implement `DatabaseUnitOfWork`: take `xpg.Beginner`, open a `*sql.Tx`, construct the outbox publisher and repositories from it.
4. Implement `InMemUnitOfWork` for test use, sharing the same in-memory repository instances.
5. Inject the `UnitOfWork` interface into command handlers. Query handlers that do not write may receive a plain `xpg.Querier` directly.
