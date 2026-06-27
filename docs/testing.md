# Testing Conventions

## Black-box packages

Always use the `package foo_test` naming convention (external test package) rather than `package foo`. This prevents tests from accessing unexported identifiers and ensures they only exercise the public API. If a test requires access to an internal type, that is a signal that the type should either be exported or the test should be redesigned.

```go
// Good
package xhttpsrv_test

// Bad — white-box, can access unexported types
package xhttpsrv
```

## Integration test coverage

Every method on a storage that interacts with an external system (Postgres, etc.) must have a corresponding integration test with 100% coverage. When adding a new method to a `*_postgres.go` storage, always add its test case to the matching `*_postgres_test.go` file in the same package before considering the work done.

The `xtestc` package provides testcontainers-go helpers (Docker required):
- `BootstrapPostgres()` — starts a PostgreSQL container with auto-migrations
- `SetupTest()` — cleans DB state between tests
- `TearDownSuite()` — container cleanup

## Explicit Test Dependencies

Never hide IDs or values behind zero-argument helpers. Every test should make its inputs obvious at the call site:

```go
// Good — explicit values visible at the call site
result := seedEntity("entity-123")

// Bad — implicit values hidden inside the helper
result := seedEntity() // caller has no idea what values were used
```

## Test Assertion Helpers

Tests should be as clean and readable as production code. When an assertion is non-trivial or reused, extract it into a named helper rather than inlining an if/else block. Use an early return for the zero/empty case so the happy path stays unindented:

```go
// Good — named helper, early return, no if/else at call site
func assertEqualDescriptions(s *testSuite, expected map[string]string, actual *map[string]string) {
    s.T().Helper()
    if len(expected) == 0 {
        s.Empty(actual)
        return
    }
    s.Require().NotNil(actual)
    s.Equal(expected, *actual)
}

// Bad — inline if/else clutters the caller
if len(e.Descriptions) > 0 {
    s.Require().NotNil(got.Descriptions)
    s.Equal(e.Descriptions, *got.Descriptions)
} else {
    s.Nil(got.Descriptions)
}
```

The helper name documents intent; the early return flattens branching; the caller stays a single line.

## Test Structure and Modularity

Treat test code with the same care as production code. Tests must be **readable and clean** — avoid large blocks of procedural logic inside `t.Run` bodies.

Extract every distinct operation into a named closure defined at the top of the parent test function. The `t.Run` body should read like a sequence of named steps, with no raw logic:

```go
// Good — closures define operations; t.Run body is a clean narrative
func TestFoo(t *testing.T) {
    buildClient := func(t *testing.T) *Client { ... }
    createResource := func(ctx context.Context, t *testing.T, client *Client, id string) { ... }
    deleteResource := func(ctx context.Context, t *testing.T, client *Client, id string) { ... }

    t.Run("should return the resource by id", func(t *testing.T) {
        ctx := t.Context()
        client := buildClient(t)
        createResource(ctx, t, client, "resource-123")
        t.Cleanup(func() { deleteResource(ctx, t, client, "resource-123") })

        result, err := client.Get(ctx, "resource-123")
        require.NoError(t, err)
        assert.Equal(t, "resource-123", result.ID)
    })
}

// Bad — raw logic clutters the test body, intent is obscured
func TestFoo(t *testing.T) {
    t.Run("should return the resource by id", func(t *testing.T) {
        sess, err := session.NewSession(...)
        require.NoError(t, err)
        client := NewClient(sess)
        payload, _ := json.Marshal(map[string]string{"id": "resource-123"})
        _, err = client.raw.Create(&CreateInput{Body: string(payload)})
        require.NoError(t, err)
        ...
    })
}
```

Guidelines:
- **One closure per concern**: building clients, seeding data, cleaning up, asserting complex outcomes.
- **Named variables over inline literals**: `key := "db-password"` and `insertedValue := "s3cret"` rather than bare string literals at the call site.
- **Closures defined before sub-tests**, so they are reusable across multiple `t.Run` blocks within the same parent.
- **`t.Helper()` on every helper** so test failure lines point to the call site, not inside the helper.

## Error Assertions

Never assert on error string content. Use sentinel errors and `errors.Is` / `assert.ErrorIs` instead. This makes tests resilient to message changes and ensures errors are checked by identity, not by fragile substring matching.

```go
// Good — sentinel error with errors.Is
var ErrInvalidConfig = errors.New("invalid config")

assert.ErrorIs(t, err, ErrInvalidConfig)

// Bad — string content assertion
assert.Contains(t, err.Error(), "invalid config")
```

## Test Helper File Placement

`make deadcode` uses a two-pass strategy to distinguish test helpers from dead production code. For this to work, test helpers must be placed according to their scope:

- **Helpers used only within one package** — go in a `_test.go` file in that package (not importable, but deadcode handles them correctly).
- **Helpers shared across packages, within the same `internal/` package** — go in a file named `testing.go` inside that package (e.g. `xtime/testing.go`, `xid/testing.go`). The filename is the signal that deadcode uses to apply the test pass.
- **Helpers that span many unrelated packages** — go in a dedicated package (currently `internal/shared/xtestc`), listed in `DEADCODE_TEST_PKGS` in the Makefile.

Never put test utilities in files with production names (e.g. `xtime/time.go`) — they will be treated as production code by the deadcode tool and flagged if no production caller exists.

## Test Helpers and Encapsulation

Never bypass encapsulation in tests by mutating an object's internal state directly (e.g. holding a reference to the underlying map passed to a constructor and modifying it). If a test needs to mutate state on a type, that type must expose a method for it. This keeps tests honest about the public API and prevents them from relying on implementation details.

```go
// Good — uses an explicit method on the type
sm.UpdateSecret("key", "new-value")

// Bad — mutates the underlying map that was passed to the constructor
secrets["key"] = "new-value"
```
