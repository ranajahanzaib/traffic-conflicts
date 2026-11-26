.PHONY: dev api test validate format build demo-data frontend-build frontend-lint frontend-dev

DEVELOPER_DIR ?= /Library/Developer/CommandLineTools
export DEVELOPER_DIR
CGO_ENABLED ?= 0
export CGO_ENABLED
GOCACHE ?= $(CURDIR)/.cache/go-build
export GOCACHE

dev: demo-data frontend-build
	cd services/api && go run ./cmd/atlas

api: demo-data frontend-build
	cd services/api && go run ./cmd/atlas

frontend-build:
	pnpm --dir apps/web build

frontend-lint:
	pnpm --dir apps/web lint

# Run `make api` in another terminal to serve the API proxy target on port 8080.
frontend-dev:
	pnpm --dir apps/web dev

test:
	go -C services/api test ./...
	python3 -m unittest discover -s pipelines/tests -v

validate:
	python3 -m pipelines.traffic_conflicts.cli sources-list

format:
	gofmt -w services/api/cmd/atlas/*.go

build:
	pnpm --dir apps/web build
	go -C services/api build -o $(CURDIR)/.cache/atlas ./cmd/atlas

# The bundled fixture is deterministic synthetic geometry; it is not research data.
demo-data:
	python3 -m pipelines.traffic_conflicts.indexing --output data/index.sqlite --demo-fixture data/fixtures/synthetic-crossing.json
