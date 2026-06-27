# Build & Development Commands

## Protobuf

```bash
# Lint + generate + tidy
make proto
```

## Database Migrations

```bash
# Interactive (requires fzf)
make migration

# Non-interactive (bot-friendly)
# db= accepts slash-separated paths for subdomain DBs: db=reservations/seating
make migration-non-interactive db=<db_name> name=<snake_case_name>

# Run migrations against postgres
# Flat:      MIGRATIONS_DIR=example_db
# Subdomain: MIGRATIONS_DIR=reservations/seating
make migrate
```

## Service Generation

```bash
# Scaffold a new service (http_api, grpc_api, worker, or job)
# module may be nested: reservations/seating → internal/{module}/cmd/{type}/
make genservice module=<module_path> type=<http_api|grpc_api|worker|job>

# With an optional feature prefix → internal/{module}/cmd/{feature}_{type}/
make genservice module=<module_path> feature=<feature_name> type=<http_api|grpc_api|worker|job>
```

See [architecture.md](architecture.md) for what the scaffold produces.

## Testing

```bash
# Run all tests (requires Docker for testcontainers)
make test

# Run mutation testing (slow — requires gremlins)
make mutation-test-all
# Faster: only packages changed by the current git diff
make mutation-test-changed

# Run a single test
go test ./internal/shared/xtestc/ -run TestPostgres

# Run tests in a specific package
go test ./internal/shared/xconfigs/
```

## Config Validation

```bash
# Validate all service configs across all environments (dev, staging, prod)
make testconfig-all

# Validate a single service (path separators replaced with dashes)
make testconfig-internal-reservations-cmd-grpc_api
```

## OpenAPI

```bash
# After editing an oapi.spec.yaml, regenerate server code
go generate ./contracts/oapi/<module>/<feature>/
# Example:
go generate ./contracts/oapi/reservations/seating/
```

See [contracts.md](contracts.md) for the OpenAPI workflow.

## AsyncAPI

```bash
# Regenerate all AsyncAPI code
make asyncapi

# Regenerate a single spec
go generate ./contracts/asyncapi/public/reservations/
```

See [contracts.md](contracts.md) for the AsyncAPI workflow.

## Protobuf Linting

```bash
cd contracts/proto && buf lint
```
