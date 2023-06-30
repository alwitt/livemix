SHELL = bash

all: build

.PHONY: lint
lint: .prepare ## Lint the files
	@go mod tidy
	@golint ./...
	@golangci-lint run ./...

.PHONY: fix
fix: .prepare ## Lint and fix vialoations
	@go mod tidy
	@go fmt ./...
	@golangci-lint run --fix ./...

.PHONY: mock
mock: .prepare ## Generate test mock interfaces
	@mockery --dir db --name PersistenceManager
	@mockery --dir utils --name SegmentReader
	@mockery --dir utils --name VideoSegmentCache
	@mockery --dir vod --name PlaylistBuilder
	@mockery --dir vod --name SegmentManager
	@mockery --dir api --name RequestResponseClient
	@mockery --dir control --name SystemManager

.PHONY: test
test: .prepare ## Run unittests
	@go test --count 1 -timeout 30s -short ./...

.PHONY: one-test
one-test: .prepare ## Run one unittest
	@go test --count 1 -v -timeout 30s -run ^$(FILTER) github.com/alwitt/livemix/...

.PHONY: build
build: lint ## Build the application
	@go build -o stream-hub.bin .

.PHONY: compose
compose: ## Prepare the development docker stack
	@docker-compose -f docker/docker-compose.yaml up -d

.PHONY: clean
clean: ## Clean up development environment
	@docker-compose -f docker/docker-compose.yaml down

.PHONY: ctrl
ctrl: build ## Run local development system control node application
	. .env; ./stream-hub.bin ctrl -c tmp/dev-ctrl-node-cfg.yaml -p postgres

.PHONY: edge
edge: build ## Run local development edge node application
	. .env; ./stream-hub.bin edge -c tmp/dev-edge-node-cfg.yaml

.prepare: ## Prepare the project for local development
	@pip3 install --user pre-commit
	@pre-commit install
	@pre-commit install-hooks
	@GO111MODULE=on go install -v github.com/go-critic/go-critic/cmd/gocritic@latest
	@GO111MODULE=on go get -v -u github.com/swaggo/swag/cmd/swag
	@touch .prepare

help: ## Display this help screen
	@grep -h -E '^[a-z0-9A-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
