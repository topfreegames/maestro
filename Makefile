SOURCES := $(shell \
	find . -not \( \( -name .git -o -name .go -o -name vendor -o -name '*.pb.go' -o -name '*.pb.gw.go' -o -name '*_gen.go' -o -name '*mock*' \) -prune \) \
	-name '*.go')

.PHONY: help
help: Makefile ## Show list of commands.
	@echo "Choose a command to run in "$(APP_NAME)":"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

# hidden targets (that will not be shown in help)

.PHONY: setup
setup: ## Download the dependencies to the project.
	@go get ./...
	@go mod download

.PHONY: setup/e2e
setup/e2e: ## Download e2e dependencies to local environment.
	@cd e2e; go mod download

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

.PHONY: run/tests
run/tests: run/unit-tests run/integration-tests ## Execute all unit and integration tests

.PHONY: run/unit-tests
run/unit-tests: ## Execute unit tests.
	@go test -count=1 -tags=unit -coverprofile=coverage.out -covermode=atomic $(OPTS) ./...

.PHONY: run/integration-tests
run/integration-tests: ## Execute integration tests.
	@go test -tags=integration -count=1 -timeout 20m -coverprofile=coverage.out -covermode=atomic $(OPTS) ./...

.PHONY: run/runtime-integration-tests
run/runtime-integration-tests: ## Execute runtime integration tests.
	@go test -tags=integration -count=1 -timeout 20m ./internal/adapters/runtime/kubernetes/...

.PHONY: license-check
license-check: ## Execute license check.
	@go run github.com/google/addlicense -skip yaml -skip yml -skip proto -check .

.PHONY: run/e2e-tests
run/e2e-tests: build-linux-x86_64 setup/e2e deps/stop ## Execute end-to-end tests.
	@cd e2e; go test -count=1 -v $(OPTS) ./suites/...

#-------------------------------------------------------------------------------
#  Build and run
#-------------------------------------------------------------------------------

.PHONY: build
build: build-linux-x86_64 ## Build the project and generates a binary.
	@rm -f ./bin/maestro || true
	@go build -o ./bin/maestro ./

.PHONY: build-linux-x86_64
build-linux-x86_64: ## Build the project and generates a binary for x86_64 architecture.
	@rm -f ./bin/maestro-linux-x86_64 || true
	@env GOOS=linux GOARCH=amd64 go build -o ./bin/maestro-linux-x86_64 ./


.PHONY: run/worker
run/worker: ## Runs maestro worker.
	@go run main.go start worker -l development

.PHONY: run/management-api
run/management-api: build ## Runs maestro management-api.
	@go run main.go start management-api -l development

.PHONY: run/rooms-api
run/rooms-api: build ## Runs maestro rooms-api.
	@go run main.go start rooms-api -l development

.PHONY: run/runtime-watcher
run/runtime-watcher: build ## Runs maestro runtime-watcher.
	@go run main.go start runtime-watcher -l development


.PHONY: run/metrics-reporter
run/metrics-reporter: build ## Runs maestro metrics-reporter.
	@go run main.go start metrics-reporter -l development

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
	@go run main.go migrate

#-------------------------------------------------------------------------------
#  Local dependencies
#-------------------------------------------------------------------------------

.PHONY: deps/up
deps/up: ## Create containers dependencies.
	@echo "Creating dependencies "
	@docker-compose --project-name maestro up -d
	@echo "Dependencies created successfully."

.PHONY: deps/start
deps/start: ## Start containers dependencies.
	@echo "Starting dependencies "
	@docker-compose --project-name maestro start
	@echo "Dependencies started successfully."

.PHONY: deps/stop
deps/stop: ## Stop containers dependencies.
	@echo "Stopping dependencies "
	@docker-compose --project-name maestro stop
	@echo "Dependencies stopped successfully."

.PHONY: deps/down
deps/down: ## Delete containers dependencies.
	@echo "Deleting dependencies "
	@docker-compose --project-name maestro down
	@echo "Dependencies deleted successfully."


#-------------------------------------------------------------------------------
#  Easily start Maestro
#-------------------------------------------------------------------------------
.PHONY: maestro/start
maestro/start: build-linux-x86_64 ## Start Maestro with all of its dependencies
	@echo "Starting maestro..."
	@cd ./e2e/framework/maestro; docker-compose --project-name maestro up -d
	@go run main.go migrate;
	@cd ./e2e/framework/maestro; docker-compose --project-name maestro start
	@echo "Maestro is up and running!"
