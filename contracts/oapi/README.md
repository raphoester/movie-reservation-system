# OpenAPI specs

One directory per feature: `contracts/oapi/<module>/<feature>/`, each containing
`oapi.spec.yaml`, `cfg.yaml`, and a `generate.go` (`go generate ./contracts/oapi/<module>/<feature>/`).
Generated server code lands in a sibling `<module>_<feature>/` package and is never edited by hand.

See [docs/contracts.md](../../docs/contracts.md).
