SOURCES := $(shell \
	find . -not \( \( -name .git -o -name .go -o -name vendor -o -name '*.pb.go' -o -name '*.pb.gw.go' -o -name '*_gen.go' -o -name '*mock*' \) -prune \) \
	-name '*.go')

.PHONY: help
help: Makefile ## Show list of commands.
	@echo "Choose a command to run in "$(APP_NAME)":"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

# hidden targets (that will not be shown in help)

.PHONY: deps
deps: ## Download the dependencies to the project.
	@go get ./...
	@go mod download

#-------------------------------------------------------------------------------
#   Lint and tests
#-------------------------------------------------------------------------------

.PHONY: goimports
goimports: ## Execute goimports to standardize modules declaration and code. 
	@go run golang.org/x/tools/cmd/goimports -w $(SOURCES)

.PHONY: lint
lint: lint/go lint/protobuf ## Execute linters.

.PHONY: lint/go
lint/go: ## Execute golangci-lint. 
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: lint/protobuf
lint/protobuf: ## Execute buf linter.
	@go run github.com/bufbuild/buf/cmd/buf lint --config buf.yaml

.PHONY: run/unit-tests
run/unit-tests: ## Execute unit tests.
	@go test -count=1 -tags=unit -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: run/integration-tests
run/integration-tests: ## Execute integration tests.
	@go test -tags=integration -count=1 -timeout 20m -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: run/runtime-integration-tests
run/runtime-integration-tests: ## Execute runtime integration tests.
	@go test -tags=integration -count=1 -timeout 20m ./internal/adapters/runtime/kubernetes/...

.PHONY: license-check
license-check: ## Execute license check.
	@go run github.com/google/addlicense -skip yaml -skip yml -skip proto -check .

.PHONY: run/e2e-tests
run/e2e-tests: deps/stop build/worker build/management-api build/rooms-api build/runtime-watcher ## Execute end-to-end tests.
	cd e2e; go mod download; go test -count=1 ./suites/...

#-------------------------------------------------------------------------------
#  Build and run
#-------------------------------------------------------------------------------

.PHONY: build/all-components 
build/all-components: build/worker build/management-api build/rooms-api build/runtime-watcher ## Build all maestro components binaries (management-api, worker, rooms-api and runtime-watcher).
	@echo 'Built all components (worker, management-api, rooms-api, runtime-watcher)! Look in the bin folder'

.PHONY: build/worker
build/worker: ## Build worker binary.
	@rm -f ./bin/worker-* || true
	@go build -o ./bin/worker ./cmd/worker
	@env GOOS=linux GOARCH=amd64 go build -o ./bin/worker-linux-x86_64 ./cmd/worker

.PHONY: run/worker
run/worker: build/worker
	./bin/worker

.PHONY: build/management-api
build/management-api: ## Build management-api binary.
	@rm -f ./bin/management-api-* || true
	@go build -o ./bin/management-api ./cmd/management_api
	@env GOOS=linux GOARCH=amd64 go build -o ./bin/management-api-linux-x86_64 ./cmd/management_api

.PHONY: build/rooms-api
build/rooms-api: ## Build rooms-api binary.
	@rm -f ./bin/rooms-api-* || true
	@go build -o ./bin/rooms-api ./cmd/rooms_api
	@env GOOS=linux GOARCH=amd64 go build -o ./bin/rooms-api-linux-x86_64 ./cmd/rooms_api

.PHONY: build/runtime-watcher
build/runtime-watcher: ## Build runtime-watcher binary.
	@rm -f ./bin/runtime-watcher-* || true
	@go build -o ./bin/runtime-watcher ./cmd/runtime_watcher
	@env GOOS=linux GOARCH=amd64 go build -o ./bin/runtime-watcher-linux-x86_64 ./cmd/runtime_watcher

.PHONY: run/management-api
run/management-api: build/management-api  ## Run management-api.
	./bin/management-api

.PHONY: run/rooms-api
run/rooms-api: build/rooms-api ## Run rooms-api.
	./bin/rooms-api

.PHONY: run/runtime-watcher
run/runtime-watcher: build/runtime-watcher ## Run runtime-watcher.
	./bin/runtime-watcher

#-------------------------------------------------------------------------------
#  Code generation
#-------------------------------------------------------------------------------

.PHONY: generate
generate: ## Execute code generation.
	@go generate ./gen

#-------------------------------------------------------------------------------
#  Migration and database make targets
#-------------------------------------------------------------------------------

.PHONY: migrate
migrate: ## Execute migration.
	@go run cmd/utils/utils.go migrate

#-------------------------------------------------------------------------------
#  Local dependencies
#-------------------------------------------------------------------------------

.PHONY: deps/start
deps/start: ## Start containers dependencies.
	@echo "Starting dependencies "
	@docker-compose --project-name maestro up -d
	@echo "Dependencies started successfully."

.PHONY: deps/stop
deps/stop: ## Stop containers dependencies.
	@echo "Stopping dependencies "
	@docker-compose --project-name maestro down
	@echo "Dependencies stopped successfully."

.PHONY: deps/down
deps/down: ## Delete containers dependencies.
	@echo "Stopping dependencies "
	@docker-compose --project-name maestro down
	@echo "Dependencies stopped successfully."
