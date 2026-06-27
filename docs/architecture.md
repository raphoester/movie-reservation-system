# Architecture

Go microservice framework with shared infrastructure packages and a code generator for scaffolding new services.

## Core Layout

```
internal/shared/            Reusable framework packages ("the framework")
internal/{module}/cmd/{service}/   Individual service entry points
internal/tooling/cmd/genservice/   Code generator for new services
contracts/proto/            Protobuf definitions (versioned: v1, v2) with buf tooling
contracts/oapi/             OpenAPI specs and generated server code
contracts/asyncapi/         AsyncAPI event schemas (public + private)
assets/migrations/{db}/     SQL migration files (embedded via assets/fs.go)
assets/configs/             Shared YAML config anchors (embedded via assets/fs.go)
scripts/                    Shell scripts for CI and config validation
```

## Bootstrap Pattern

```
main.go → xconfigs.Load[Config]() → xlog.New() → xbootstrap.HttpServer[Config]() or GrpcServer[Config]()
```

Bootstrap handles server creation, middleware, graceful shutdown (SIGINT/SIGTERM), and optional DataDog tracing (non-dev only).

## Shared Packages (`internal/shared/`)

| Package       | Purpose                                                             |
| ------------- | ------------------------------------------------------------------- |
| `xbootstrap` | Generic server bootstrap with DI callbacks                          |
| `xconfigs`   | Viper-based YAML config loading per environment with shared anchors |
| `xhttpsrv`   | HTTP server setup (stdlib `http.ServeMux`) with logging middleware  |
| `xgrpcsrv`   | gRPC server with logging, recovery, and cancellation interceptors   |
| `xlog`       | Structured JSON logging via `slog`                                  |
| `xpg`        | PostgreSQL connection (GORM) + migration runner                     |
| `xtestc`     | Testcontainers-go helpers for integration tests                     |
| `xvalidate`  | go-playground/validator wrapper                                     |
| `xtime`      | Time provider abstraction                                           |
| `xid`        | ID generation utilities                                             |
| `xos`        | OS signal context handling                                          |
| `xcolls`     | Generic collections (Set)                                           |
| `xgeo`       | Geo utilities                                                       |
| `xhealth`    | Health checks and monitoring                                        |
| `xmessaging` | Messaging configuration (Watermill-based)                           |
| `xotel`      | OpenTelemetry SDK configuration                                     |
| `ontelemetry` | OpenTelemetry instrumentation (logger, meter, tracer, propagator)   |

## Service Generation

`make genservice` scaffolds a complete service with:

- `main.go` with bootstrap wiring
- `config.go` implementing required config interfaces
- `Dockerfile` (multi-stage Alpine build)
- Per-environment YAML configs (with shared anchor references), including `local.yaml`

Ports are auto-assigned by scanning existing `local.yaml` files (HTTP from 3000, gRPC from 50051).

See [commands.md](commands.md) for the full `genservice` command syntax.
