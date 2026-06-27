# OpenAPI specs

One directory per feature: `contracts/oapi/<context>/<feature>/`, each containing
`oapi.spec.yaml`, `cfg.yaml`, and a `generate.go` (`go generate ./contracts/oapi/<context>/<feature>/`).
Generated server code lands in a sibling `<context>_<feature>/` package and is never edited by hand.

See [docs/contracts.md](../../docs/contracts.md).
