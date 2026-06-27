# Configuration System

## File Layout

- **Shared anchors** live in `assets/configs/` (embedded into binaries via `embed.FS`):
  - `base.yaml` — environment-independent anchors (server port defaults)
  - `{env}.yaml` — environment-specific anchors (logger level, postgres defaults)
- **Service configs** live in `internal/{context}/cmd/{service}/configs/{env}.yaml`

At load time the loader concatenates shared YAML (base + env) with the service YAML, so anchors defined in shared files resolve in service files. Shared keys are prefixed with `_` (e.g. `_logger_defaults`) and ignored during unmarshal.

## Environments

- Environment is determined by the `ENVIRONMENT` env var (defaults to `dev`).
- `dev`, `staging`, `prod` are cloud environments.
- `local` / `tmp` is the Docker Compose environment (see [local-dev.md](local-dev.md)).

## Example Service Config

```yaml
name: my_service
version: "1.0.0"
logger:
  <<: *logger_defaults
server:
  <<: *http_server_defaults
  urlPrefix: /my-service
```

Available shared anchors include `*logger_defaults`, `*http_server_defaults`, etc.

## Shared Infrastructure Anchors

Services that communicate through infrastructure-backed queues must use the
same shared anchor for that dependency in a given environment. For example,
`reservations-worker` publishes seat-hold events to Redis, and a notifications
consumer reads those events from Redis. The reservations worker must therefore
use the environment's shared `*cache` anchor instead of a service-specific or
legacy Redis endpoint.

When changing Redis endpoints, update the shared anchor in
`assets/configs/{env}.yaml` and redeploy every service that embeds that config
into its image. A config-only change is not live until the affected Docker image
is rebuilt and the service is redeployed with it.

## Config Structs

Config structs implement interfaces: `AppConfig`, `HTTPServerConfig`, `GRPCServerConfig`. All fields are validated on load via struct tags.

Setting `VALIDATE_CONFIG_ONLY=1` makes the loader validate and exit without starting the service — used by CI and `make testconfig-*`.

## Conventions

**`mapstructure` tags** are only needed when the YAML key cannot be matched to the Go field name by case-insensitive comparison (e.g. `media_bucket` → `Bucket` needs a tag, but `baseUrl` → `BaseURL` does not). Do not add tags when viper's default matching already works.

**`awssm://` prefix** is only for genuinely confidential values: passwords, API tokens, private keys. Non-sensitive values (bucket names, regions, base URLs, feature flags) must be plain text.
