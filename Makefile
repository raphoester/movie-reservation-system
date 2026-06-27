.PHONY: proto asyncapi lint-asyncapi migration migration-non-interactive genservice setup-ci-tools setup-tools setup-mac migrate test integration-test mutation-test-all mutation-test-changed testconfig-all testconfig-% verify-secrets-all verify-secrets-% fill-missing-secrets-all fill-missing-secrets-% lint-ci render-configs local-up local-down local-migrate local-services local-list local-stop build-images tidy tidy-full test-format install-hooks deadcode check-orphans aws-check-auth aws-ensure-auth

NCPU := $(shell go run ./internal/tooling/cmd/gomaxprocs)
MAX_PARALLEL ?= $(NCPU)

MODULE := $(shell go list -m)
MAIN_PKGS := $(shell go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... | sed 's|^$(MODULE)/|./|')

# All deployable services: internal dirs that have both a Dockerfile and a main.go.
SERVICES := $(shell for f in $$(find internal -name Dockerfile -type f | sort); do \
	d=$$(dirname $$f); [ -f "$$d/main.go" ] && echo "$$d"; \
done)
SERVICE_TARGETS := $(subst /,-,$(SERVICES))

migration:
	@./scripts/create_migration.sh

migrate:
	cd ./internal/tooling/cmd/migrate && go run .

migration-non-interactive:
	@./scripts/create_migration.sh db=$(db) name=$(name)

proto:
	@echo "Generating protobuf files..."
	@cd ./contracts/proto && ../../scripts/proto.sh

asyncapi:
	@echo "Generating AsyncAPI code..."
	@go generate ./contracts/asyncapi/...
	@$(MAKE) lint-asyncapi

lint-asyncapi:
	@go run ./internal/tooling/cmd/lint_asyncapi

LINT_EXTRA_ARGS?=
lint-ci: lint-asyncapi 
	@golangci-lint run --timeout=10m $(LINT_EXTRA_ARGS)

check-orphans: ## Detect packages unreachable from any main or test; exits non-zero if orphans are found
	@go run ./internal/tooling/cmd/check_orphans -tags integration

deadcode: ## Two-pass deadcode: flags dead production code (incl. test-only callers) and unused test helpers
	@prod=$$(deadcode -tags integration,e2e ./... | grep -vE "(/[^/]*test[^/]*/|/testing\.go:|_testing\.go:)" || true); \
	helpers=$$(deadcode -test -tags integration,e2e ./... | grep -E "(/[^/]*test[^/]*/|/testing\.go:|_testing\.go:)" || true); \
	output=$$(printf '%s\n%s' "$$prod" "$$helpers" | grep -v '^$$' || true); \
	if [ -n "$$output" ]; then \
		echo "$$output"; \
		exit 1; \
	fi

genservice:
	go run ./internal/tooling/cmd/genservice \
		-path=$(CURDIR) \
		-module=$(module) \
		-feature=$(feature) \
		-type=$(type)

setup-ci-tools:
	go install mvdan.cc/gofumpt@v0.10.0
	go install golang.org/x/tools/cmd/deadcode@v0.45.0

setup-tools: setup-ci-tools
	# oapi-codegen is needed locally because of dependencies graph issues when installing it regularly through the go toolchain.
	# might be good to vendor/dockerize it in the future.
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
	go install github.com/lerenn/asyncapi-codegen/cmd/asyncapi-codegen@v0.46.3
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.10.1

build-images:
	@./scripts/build_images.sh

setup-mac: setup-tools build-images
	brew tap go-gremlins/tap
	brew install gremlins

install-hooks: ## Configure git to use the shared hooks in .githooks/
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .githooks/pre-push .githooks/post-checkout
	@echo "Git hooks installed. Pre-commit: tidy + format + lint. Pre-push: test."

test:
	go test -p $(NCPU) ./...

mutation-test-changed:
	$(eval CHANGED := $(shell git diff --name-only HEAD | grep '\.go$$' | xargs -I{} dirname {} | sort -u | sed 's|^|./|' | tr '\n' ' '))
	@if [ -z "$(CHANGED)" ]; then exit 0;

	echo "Running mutation tests on changed packages:"; \
	echo "$(CHANGED)"; \
	gremlins unleash $(CHANGED); \

mutation-test-all:
	gremlins unleash

integration-test:
	go test -tags=integration ./...

testconfig-all:
	@$(MAKE) -j $(MAX_PARALLEL) $(addprefix testconfig-,$(SERVICE_TARGETS))

testconfig-%:
	@APP="$(subst -,/,$*)" ./scripts/test_config.sh

AWS_PROFILE ?= default
AWS_SSO_SESSION ?= default

aws-check-auth: ## Exit 0 if the AWS SSO session is valid, non-zero otherwise
	@aws sts get-caller-identity --profile $(AWS_PROFILE) > /dev/null 2>&1

aws-ensure-auth: ## Log in via AWS SSO if the session has expired
	@$(MAKE) aws-check-auth || aws sso login --sso-session $(AWS_SSO_SESSION)

verify-secrets-all:
	@$(MAKE) -j $(MAX_PARALLEL) $(addprefix verify-secrets-,$(SERVICE_TARGETS))

verify-secrets-%:
	@go run ./internal/tooling/cmd/verify_secrets --service "$(subst -,/,$*)"

fill-missing-secrets-all:
	@go run ./internal/tooling/cmd/fill_missing_secrets $(addprefix --service ,$(SERVICES)) --env "$(or $(ENV),all)"

fill-missing-secrets-%:
	@go run ./internal/tooling/cmd/fill_missing_secrets --service "$(subst -,/,$*)" --env "$(or $(ENV),dev)"

render-configs: ## Generate {env}.resolved.yaml alongside each service's configs/ (run before local-up)
	@go run ./internal/tooling/cmd/render_configs

local-migrate: ## Re-run migrations (e.g. after adding a new migration file)
	docker compose up --force-recreate migrator

ARGS ?=
local-up: render-configs ## Start infra + all services in Docker (builds images). Extra flags: make local-up ARGS="--build"
	docker compose -f docker-compose.yml up -d $(ARGS)
	docker compose -f docker-compose.yml wait migrator
	docker compose -f docker-compose.yml -f docker-compose.services.yml up $(ARGS)

local-down: ## Stop all Docker Compose services (infra + application services)
	docker compose -f docker-compose.yml -f docker-compose.services.yml down $(ARGS)

local-services: ## Start all services (requires docker-compose up). Optionally filter by subpath: make local-services path=internal/reservations
	@bash scripts/local_services.sh $(path)

local-list: ## List currently running local services
	@[ -f local/.pids ] || (echo "no services running"; exit 0)
	@awk -F': ' '{print $$1}' local/.pids

XARGS_MAX_PROCS ?= $(shell if [ "$$(uname)" = "Darwin" ]; then sysctl -n hw.ncpu; elif [ "$$(uname)" = "Linux" ]; then nproc --all; else echo 1; fi)
TIDY_BASE ?= $(shell git rev-parse --verify origin/develop 2>/dev/null || git rev-parse --verify develop 2>/dev/null || echo "HEAD")

tidy: ## Format Go files changed vs develop (origin/develop > develop > HEAD) and run go mod tidy
	@go mod tidy
	$(eval FILES := $(shell git diff --name-only --diff-filter=ACM $(TIDY_BASE) | grep '\.go$$' | grep -v '\.pb\.go$$' || true))
	@if [ -n "$(FILES)" ]; then \
		echo "$(FILES)" | xargs -P "$(XARGS_MAX_PROCS)" gofumpt -l -w; \
	fi

tidy-full: ## Format all Go files and run go mod tidy
	@go mod tidy
	@gofumpt -l -w .

test-format: ## Check that all Go files are formatted with gofumpt
	$(eval FILES := $(shell find . \
		-name '*.go' \
		-not -path './.claude/*' \
		-not -name '*.pb.go' \
		| xargs gofumpt -l))
	@if [ -z "$(FILES)" ]; then \
		exit 0; \
	else \
		echo "These files are not formatted, please run 'make tidy':"; \
		echo "$(FILES)"; \
		exit 1; \
	fi

local-stop: ## Stop a single running service (usage: make local-stop service=catalog_grpc_api)
	@[ -n "$(service)" ] || (echo "usage: make local-stop service=<name>"; exit 1)
	@[ -f local/.pids ] || (echo "no services running (local/.pids not found)"; exit 1)
	@pid=$$(grep "^$(service):" local/.pids | cut -d' ' -f2); \
	  [ -n "$$pid" ] || (echo "service '$(service)' not found in local/.pids"; exit 1); \
	  kill "$$pid" && \
	  sed -i.bak '/^$(service):/d' local/.pids && rm -f local/.pids.bak
