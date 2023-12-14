BASE_DIR = $(realpath .)
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
	@mockery --dir db --name ConnectionManager
	@mockery --dir utils --name SegmentReader
	@mockery --dir utils --name VideoSegmentCache
	@mockery --dir utils --name S3Client
	@mockery --dir utils --name Broadcaster
	@mockery --dir vod --name PlaylistBuilder
	@mockery --dir vod --name SegmentManager
	@mockery --dir vod --name PlaylistManager
	@mockery --dir api --name RequestResponseClient
	@mockery --dir control --name SystemManager
	@mockery --dir control --name EdgeRequestClient
	@mockery --dir edge --name VideoSourceOperator
	@mockery --dir forwarder --name SegmentSender
	@mockery --dir forwarder --name RecordingSegmentForwarder
	@mockery --dir forwarder --name LiveStreamSegmentForwarder

.PHONY: test
test: .prepare ## Run unittests
	. .env; go test --count 1 -timeout 30s -short ./...

.PHONY: one-test
one-test: .prepare ## Run one unittest
	. .env; go test --count 1 -v -timeout 30s -run ^$(FILTER) github.com/alwitt/livemix/...

.PHONY: build
build: lint ## Build the application
	@go build -o livemix.bin .
	@go build -o livemix-util.bin ./bin/util/...

.PHONY: doc
doc: .prepare ## Generate the OpenAPI spec
	@swag init -g main.go --parseDependency
	@rm docs/docs.go

.PHONY: ts-sdk
ts-sdk: .prepare ## Generate Javascript client
	@mkdir -vp tmp/sdk/ts-axios
	@docker run --rm \
	  --mount type=bind,source=$(BASE_DIR)/docs,target=/input,readonly \
	  --mount type=bind,source=$(BASE_DIR)/tmp,target=/output \
	  openapitools/openapi-generator-cli:latest-release \
	    generate -i /input/swagger.yaml -g typescript-axios -o /output/sdk/ts-axios
	@echo
	@echo '!!! IMPORTANT: The generated Typescript SDK at $(BASE_DIR)/tmp/sdk/ts-axios is owned by root !!!'
	@echo

.PHONY: compose
compose: ## Prepare the development docker stack
	@docker-compose -f docker/docker-compose.yaml up -d

.PHONY: clean
clean: ## Clean up development environment
	@docker-compose -f docker/docker-compose.yaml down

.PHONY: ctrl
ctrl: build ## Run local development system control node application
	. .env.ctrl; ./livemix.bin ctrl -c tmp/dev-ctrl-node-cfg.yaml -p postgres

.PHONY: edge-%
edge-%: build ## Run local development edge node application
	. .env.edge; ./livemix.bin edge -c tmp/dev-edge-node-$*-cfg.yaml

.prepare: ## Prepare the project for local development
	@pip3 install --user pre-commit
	@pre-commit install
	@pre-commit install-hooks
	@GO111MODULE=on go install -v github.com/go-critic/go-critic/cmd/gocritic@latest
	@GO111MODULE=on go get -v -u github.com/swaggo/swag/cmd/swag
	@touch .prepare

help: ## Display this help screen
	@grep -h -E '^[a-z0-9A-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
