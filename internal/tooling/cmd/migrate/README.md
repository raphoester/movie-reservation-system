# Migrate Tool

A CLI tool that runs PostgreSQL database migrations. It connects to a specific database based on environment variables and applies all pending migrations embedded in the binary.

## How It Works

1. Loads YAML config for the current environment (selected by `ENVIRONMENT`)
2. Reads `MIGRATIONS_DIR` to determine which database to migrate
3. Looks up the matching Postgres connection config from the `postgres` map in the YAML
4. Connects to that database and runs all pending SQL migrations from `assets/migrations/<MIGRATIONS_DIR>/`

Migrations are embedded into the binary at compile time via Go's `embed.FS` (see `assets/fs.go`). The tool uses [golang-migrate](https://github.com/golang-migrate/migrate) under the hood and applies `m.Up()` — it only moves forward, never down.

## Environment Variables

| Variable         | Required | Default | Description                                                                                   |
| ---------------- | -------- | ------- | --------------------------------------------------------------------------------------------- |
| `ENVIRONMENT`    | No       | `dev`   | Selects which config file to load: `dev`, `staging`, or `prod`                                |
| `MIGRATIONS_DIR` | Yes      | —       | The subdirectory name under `assets/migrations/` **and** the key in the `postgres` config map |

`MIGRATIONS_DIR` serves a dual purpose: it selects both the SQL files directory and the database connection. The value must exactly match:

1. A directory name under `assets/migrations/` in the source code — this is where the `.sql` migration files live
2. A key in the `postgres` map of the config file — this is how the tool resolves which database to connect to

The SQL files are **embedded into the binary at build time**, so the available values are fixed at compile time. To see which values are valid, check the directory listing of `assets/migrations/`:

```
assets/migrations/
  example_db/      → MIGRATIONS_DIR=example_db
```

If `MIGRATIONS_DIR` is set to a value that doesn't match both a directory in `assets/migrations/` and a key in the config's `postgres` map, the tool will fail at startup.

## Configuration

Config files live in `configs/` next to `main.go`:

```
configs/
  local.yaml
  dev.yaml
  staging.yaml
  prod.yaml
```

Each config defines a `postgres` map where keys are migration directory names and values are Postgres connection configs:

```yaml
name: migrator
version: "1.0.0"
logger:
  <<: *logger_defaults

postgres:
  example_db:
    <<: *example_db_postgres
```

The config loader concatenates shared YAML anchors from `assets/configs/` (both `base.yaml` and `{env}.yaml`) with the service config, so anchors like `*example_db_postgres` and `*logger_defaults` resolve at load time.

### Postgres Config Fields

Each entry in the `postgres` map has these fields:

| Field      | Description                          | Example             |
| ---------- | ------------------------------------ | ------------------- |
| `host`     | Database host                        | `localhost`         |
| `port`     | Database port                        | `5432`              |
| `user`     | Database user                        | `postgres`          |
| `password` | Database password (supports secrets) | `awssm://secretRef` |
| `dbName`   | Database name                        | `example_db`        |
| `sslMode`  | SSL mode                             | `disable`           |

In non-local environments, `password` values using the `awssm://` scheme are resolved from AWS Secrets Manager at load time.

## Adding a New Database

To migrate a new database (e.g. `bookings`):

1. Create the migration files directory: `assets/migrations/bookings/`
2. Add SQL migration files using the `migration` makefile target: `make migration` at the project root
3. Add a Postgres anchor in each shared env config (`assets/configs/{env}.yaml`):
   ```yaml
   _bookings_postgres: &bookings_postgres
     host: ...
     port: 5432
     ...
   ```
4. Add the new key to the `postgres` map in each migrate config (`configs/{env}.yaml`):
   ```yaml
   postgres:
     example_db:
       <<: *example_db_postgres
     bookings:
       <<: *bookings_postgres
   ```
5. Run with `MIGRATIONS_DIR=bookings make migrate`

## Running Locally

```bash
# Migrate the example_db database in dev
MIGRATIONS_DIR=example_db make migrate

# Migrate against staging (must be logged in with AWS credentials if using awssm secrets)
ENVIRONMENT=staging MIGRATIONS_DIR=example_db make migrate
```

## Docker / Deployment

The tool ships with a Dockerfile that produces a minimal Alpine image:

```dockerfile
FROM golang:1.26.2-alpine3.23 AS builder
# ... builds the binary ...

FROM alpine:3.22
COPY --from=builder /app/main .
COPY --from=builder '/app/internal/tooling/cmd/migrate/configs' /app/configs
CMD ["./main"]
```

At runtime, the container needs:

- `ENVIRONMENT` — to select the config file (defaults to `dev` if unset)
- `MIGRATIONS_DIR` — to select which database to migrate (required, will panic if unset)
- Network access to the target PostgreSQL instance
- If using `awssm://` secrets: AWS credentials (IAM role, env vars, etc.)

The binary expects config files at `./configs/` relative to the working directory, which the Dockerfile handles by copying them to `/app/configs`.

### Running One Migration Per Database

The tool migrates exactly one database per invocation. To migrate multiple databases, run the container once per database with a different `MIGRATIONS_DIR` value. In CI/CD, this typically means one job or task per database.
