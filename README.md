# movie-reservation-system

The reference implementation for the book *An Opinionated Approach at Backend
Engineering Using the Go Language*. A small, didactic Go microservice codebase
that demonstrates the architecture, conventions, tooling, and contract workflow
advocated in the book — applied to a deliberately simple domain (reserving seats
for movie screenings).

## Layout

```
internal/shared/                   Reusable framework packages (the "x" namespace: xbootstrap, xconfigs, xpg, …)
internal/{context}/cmd/{service}/  Individual service entry points
internal/tooling/cmd/              Developer tooling (genservice, migrate, dbcreator, …)
contracts/proto|oapi|asyncapi/     Protobuf, OpenAPI and AsyncAPI contracts
assets/migrations|configs/         Embedded SQL migrations and shared config anchors
docs/                              Architecture & conventions (start here)
```

## Quick start

```bash
make setup-tools     # install buf, oapi-codegen, asyncapi-codegen, golangci-lint, gofumpt, deadcode
make install-hooks   # pre-commit: tidy + format + lint, pre-push: tests
make local-up        # postgres + redis + dbcreator + migrator (Docker), then the services
```

Generate a new service:

```bash
make genservice context=<bounded_context> type=<http_api|grpc_api|worker|job>
```

## Documentation

See [docs/](docs/) — [architecture](docs/architecture.md), [conventions](docs/conventions.md),
[configuration](docs/configuration.md), [contracts](docs/contracts.md),
[transactions](docs/transactions.md), [local development](docs/local-dev.md).
