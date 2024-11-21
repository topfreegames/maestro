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
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.0 run

.PHONY: lint/protobuf
lint/protobuf: ## Execute buf linter.
	@go run github.com/bufbuild/buf/cmd/buf lint

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
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ./bin/maestro-linux-x86_64 ./


.PHONY: run/worker
run/worker: ## Runs maestro worker.
	@MAESTRO_INTERNALAPI_PORT=8051 go run main.go start worker -l development

.PHONY: run/runtime-watcher
run/runtime-watcher: build ## Runs maestro runtime-watcher.
	@MAESTRO_INTERNALAPI_PORT=8061 go run main.go start runtime-watcher -l development

.PHONY: run/rooms-api
run/rooms-api: build ## Runs maestro rooms-api.
	@MAESTRO_INTERNALAPI_PORT=8071 MAESTRO_API_PORT=8070 go run main.go start rooms-api -l development

.PHONY: run/management-api
run/management-api: build ## Runs maestro management-api.
	@MAESTRO_INTERNALAPI_PORT=8081 MAESTRO_API_PORT=8080 go run main.go start management-api -l development


.PHONY: run/metrics-reporter
run/metrics-reporter: build ## Runs maestro metrics-reporter.
	@MAESTRO_INTERNALAPI_PORT=8091 go run main.go start metrics-reporter -l development

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
	@MAESTRO_MIGRATION_PATH="file://internal/service/migrations" go run main.go migrate

#-------------------------------------------------------------------------------
#  Local dependencies
#-------------------------------------------------------------------------------

.PHONY: deps/up
deps/up: ## Create containers dependencies.
	@echo "Creating dependencies "
	@docker-compose up -d --build
	@echo "Dependencies created successfully."

.PHONY: deps/start
deps/start: ## Start containers dependencies.
	@echo "Starting dependencies "
	@docker-compose start
	@echo "Dependencies started successfully."

.PHONY: deps/stop
deps/stop: ## Stop containers dependencies.
	@echo "Stopping dependencies "
	@docker-compose stop
	@echo "Dependencies stopped successfully."

.PHONY: deps/down
deps/down: ## Delete containers dependencies.
	@echo "Deleting dependencies "
	@docker-compose down
	@echo "Dependencies deleted successfully."


#-------------------------------------------------------------------------------
#  Easily start Maestro
#-------------------------------------------------------------------------------
.PHONY: maestro/start
maestro/start: build-linux-x86_64 ## Start Maestro with all of its dependencies.
	@echo "Starting maestro..."
	@cd ./e2e/framework/maestro; docker-compose up --build -d
	@MAESTRO_MIGRATION_PATH="file://internal/service/migrations" go run main.go migrate;
	@cd ./e2e/framework/maestro; docker-compose up --build -d worker runtime-watcher #Worker and watcher do not work before migration, so we start them after it.
	@echo "Maestro is up and running!"

.PHONY: maestro/down
maestro/down: ## Delete Maestro and all of its dependencies.
	@echo "Deleting maestro..."
	@cd ./e2e/framework/maestro; docker-compose down
	@echo "Maestro was deleted with success!"

.PHONY: maestro/tilt
maestro/tilt: ## Start Maestro with all of its dependencies using Tilt.
	@echo "Starting maestro with Tilt..."
	@kind create cluster --config ./kind-config.yaml
	@kubectx kind-maestro
	@tilt up
	@echo "Maestro is up and running with Tilt!"

.PHONY: maestro/tilt/down
maestro/tilt/down: ## Delete Maestro and all of its dependencies using Tilt.
	@echo "Deleting maestro with Tilt..."
	@tilt down
	@kind delete cluster --name maestro
	@echo "Maestro was deleted with success using Tilt!"