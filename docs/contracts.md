# Contracts

This codebase uses three contract formats: Protobuf (gRPC), OpenAPI (HTTP), and AsyncAPI (events).

## Protobuf

- **Definitions**: `contracts/proto/{service}/{version}/`
- **Generated code**: `contracts/proto/generated/` — never edit these files
- **Tooling**: `buf` v2 with BASIC linting rules; `buf.yaml` and `buf.gen.yaml` live in `contracts/proto/`
- **Generators**: `protoc-gen-go` + `protoc-gen-go-grpc` (with `require_unimplemented_servers=false`)

```bash
make proto          # lint + generate + tidy
cd contracts/proto && buf lint   # lint only
```

## OpenAPI

- **Specs**: `contracts/oapi/{module}/{feature}/oapi.spec.yaml`
- **Generated code**: `contracts/oapi/{module}/{feature}/{package}/server.gen.go` — never edit
- Each spec directory contains a `generate.go` and a `cfg.yaml`
- Uses `oapi-codegen` with `std-http-server`, `models`, and `strict-server` generation

**Usage in handlers**: The generated types (request/response models) are used directly. The generated `ServerInterface` is **not** used for routing — routes are registered manually via `http.ServeMux` with `xhttpsrv.Handle()`.

```bash
go generate ./contracts/oapi/<module>/<feature>/
```

## AsyncAPI

- **Specs**: `contracts/asyncapi/{public|private}/{module}/asyncapi.yaml`
  - `public/` — integration events shared with other teams and languages
  - `private/` — internal events, not for external consumption
  - Both support arbitrary nesting: `public/reservations/seating/asyncapi.yaml` is valid
- **Generated code**: `contracts/asyncapi/{public|private}/{module}/{public|private}_{module}_events/asyncapi.gen.go` — never edit
- **Tooling**: `asyncapi-codegen` (`github.com/lerenn/asyncapi-codegen`), install via `make setup-tools`
- **Format**: AsyncAPI 3.0.0
- Code generation produces **types only** (`-g types`); broker wiring uses native Watermill patterns via `xmessaging`

```bash
make asyncapi                                        # regenerate all specs
go generate ./contracts/asyncapi/public/reservations/  # regenerate one spec
```

### Topic Naming Convention

Enforced by `make lint-asyncapi` (runs in pre-commit). Segments are dot-separated; module and event-name use kebab-case.

| Type    | Pattern                             | Example                                          |
|---------|-------------------------------------|--------------------------------------------------|
| Public  | `public.<module>.<event-name>`      | `public.reservations.reservation-confirmed`      |
| Private | `private.<module>.<event-name>`     | `private.reservations.seat-held`                 |

### Using Generated Types in Application Code

When publishing an event, always use the generated constants and payload structs — never duplicate them by hand.

- **Topic**: use the generated `{EventName}ChannelPath` const as the topic string.
- **Payload**: marshal using the generated `{EventName}PayloadSchema` struct.

```go
// events.go — correct pattern
import resevents "github.com/raphoester/movie-reservation-system/contracts/asyncapi/private/reservations/private_reservations_events"

func (e SeatHeld) Topic() string {
    return resevents.SeatHeldChannelPath
}

func (e SeatHeld) Marshalled() ([]byte, error) {
    return json.Marshal(resevents.SeatHeldPayloadSchema{ReservationId: e.ReservationID})
}
```

Do not add a corresponding constant in `domain/events/topics.go` — that would duplicate what the generator already provides.
