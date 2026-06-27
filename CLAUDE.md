# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`movie-reservation-system` is the **canonical example repository** for the book
*An Opinionated Approach at Backend Engineering Using the Go Language* (repo
`go-manifesto`). Every directive in the book must be illustrable by real,
compiling code that lives here. Keep the code **clean, minimal and didactic**:
it exists to demonstrate a point, not to cover every edge case.

The framework and tooling were extracted from a production codebase and made
generic. The shared packages use an **`x` prefix** (`xbootstrap`, `xconfigs`,
`xpg`, …) — think of it as "the framework namespace".

## Docs structure

Each file in `docs/` should stay under **200 lines**. If a file grows beyond that, split the overflowing section into a new `docs/` file and replace the content with a short summary and a link. Creating sub directories is permitted if it helps with organization, but is not required.

## Build & Development Commands

Full reference for all `make` and `go generate` commands (proto, migrations, service generation, tests, config validation, OpenAPI, AsyncAPI):

→ [docs/commands.md](docs/commands.md)

## Architecture

Go microservice framework with shared infrastructure packages and a code generator for scaffolding new services. Covers core layout, bootstrap pattern, shared packages (`internal/shared/`), service generation, and key dependencies:

→ [docs/architecture.md](docs/architecture.md)

## Configuration System

YAML-based config with shared anchors per environment, `mapstructure` tag conventions, and `awssm://` secret prefix rules:

→ [docs/configuration.md](docs/configuration.md)

## Contracts (Protobuf / OpenAPI / AsyncAPI)

How to work with and regenerate Protobuf, OpenAPI, and AsyncAPI contracts; topic naming conventions:

→ [docs/contracts.md](docs/contracts.md)

## Local Development

Prerequisites, first-time setup (git hooks, machine config), day-to-day workflow, adding databases:

→ [docs/local-dev.md](docs/local-dev.md)

## Coding Conventions

OO design (thin handlers, rule/command/event objects, value constructors), immutability, slice mapping, sets, comments, command/query naming, and all testing conventions (black-box packages, integration coverage, test structure, error assertions):

→ [docs/conventions.md](docs/conventions.md)

## Linting

All code must pass `make lint-ci`. Key rules — wrap external errors, call method values, use suite assertions, no nil/nil:

→ [docs/linting.md](docs/linting.md)

## Jobs

Side-effect-free core logic, logs in decorators, collect-then-report in `main.go`:

→ [docs/jobs.md](docs/jobs.md)

## Tooling Conventions

Structure rules for CLI programs under `internal/tooling/cmd/`, including flag parsing, logging, error handling, and usage messages:

→ [docs/tooling.md](docs/tooling.md)

## Transactions & Outbox Pattern

Recipe for handling database transactions and transactional event publishing (UnitOfWork, Transaction, TransactionalRepositories, EventPublisher, `xpg.Querier`):

→ [docs/transactions.md](docs/transactions.md)
