# Local Development Guide

## Prerequisites

- **Go 1.26+**
- **Docker** (with Docker Compose v2 — `docker compose`, not `docker-compose`)
- **fzf** — required for `make migration` (interactive migration creator)

---

## First-time setup

### 1. Git hooks

Activate the shared git hooks (pre-commit: tidy + format check + lint; pre-push: tests):

```bash
make install-hooks
```

Use `git commit --no-verify` / `git push --no-verify` to bypass when needed.

### 2. AWS credentials (cloud environments only)

Local development needs no cloud credentials — postgres and redis run in Docker Compose. You only need AWS access when working against `dev`, `staging`, or `prod`, where confidential config values use the `awssm://` prefix and are resolved from AWS Secrets Manager at load time.

Configure AWS credentials through your normal AWS profile or SSO login, so that the secret resolver can authenticate when a config references an `awssm://` value.

### 3. Start local infrastructure

```bash
make local-up
```

This starts **postgres** and **redis** via Docker Compose, then runs **dbcreator** and the **migrator** automatically:

- Creates all application databases
- Runs all pending SQL migrations

`local-up` is **idempotent** — safe to re-run. Migrations use version tracking so they never apply twice.

---

## Day-to-day workflow

### Start all services

```bash
make local-services
```

Discovers and starts every service that has a `configs/local.yaml` in parallel.

**Ctrl+C stops everything**

### Debug a single service

While `make local-services` is running in one terminal, you can stop and restart a single service from another:

```bash
make local-list                                    # see what's running
make local-stop service=reservations_grpc_api      # stop it
cd internal/reservations/cmd/grpc_api
ENVIRONMENT=local go run .                         # restart manually
```

This mechanic is useful for quick tests, and allows you to attach a debugger if you need.

### Re-run migrations

After adding a new migration file:

```bash
make local-migrate
```

### Clean slate

Wipe all data and start fresh:

```bash
docker compose down -v   # stop containers and remove volumes (data)
docker compose up -d       # start with clean postgres and redis
```

---

## How `local` configs work

Each service has a `configs/local.yaml` alongside the standard `dev/staging/prod` configs.
Shared anchors (postgres hosts, redis endpoints, log level) are defined in `assets/configs/local.yaml` and merged at load time.

Key characteristics of the local environment:

- **JSON logging is replaced with colored console output** — each service gets a unique ANSI color derived from its name
- **DataDog tracing is disabled**
- Services connect to postgres and redis on **localhost** (via the Docker-mapped ports)

The `local` environment is validated by `make testconfig-all` alongside the cloud environments, so config regressions are caught in CI.

---

## Generating a new service

```bash
make genservice module=<module> feature=<feature_name> type=<http_api|grpc_api|worker>
```

This scaffolds `main.go`, `config.go`, `Dockerfile`, and per-environment YAML configs including `local.yaml`.

Ports are auto-assigned: `genservice` scans all existing `local.yaml` files and picks the next available port (HTTP starting from 3000, gRPC from 50051).

---

## Adding a new database / module

1. Add the new database to `internal/tooling/cmd/dbcreator/configs/` for each environment
2. Add the connection config to `internal/tooling/cmd/migrate/configs/` for each environment
3. Create migration files under `assets/migrations/<db_name>/`
4. For local: add the anchor to `assets/configs/local.yaml` and update the dbcreator/migrate local configs
