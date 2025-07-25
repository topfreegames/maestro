SOURCES := $(shell \
	find . -not \( \( -name .git -o -name .go -o -name vendor -o -name '*.pb.go' -o -name '*.pb.gw.go' -o -name '*_gen.go' -o -name '*mock*' \) -prune \) \
	-name '*.go')

BUF := github.com/bufbuild/buf/cmd/buf@v1.32.1

# Detect Docker Compose command - prefer 'docker compose' over 'docker-compose'
DOCKER_COMPOSE := $(shell if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker-compose"; fi)

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
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5 run

.PHONY: lint/protobuf
lint/protobuf: ## Execute buf linter.
	@go run $(BUF) dep update && go run $(BUF) lint

.PHONY: run/tests
run/tests: run/unit-tests run/integration-tests ## Execute all unit and integration tests

.PHONY: run/unit-tests
run/unit-tests: ## Execute unit tests.
	@go test -count=1 -tags=unit -coverprofile=coverage.out -covermode=atomic $(OPTS) ./...

.PHONY: run/integration-tests
run/integration-tests: k3d/up ## Execute integration tests.
	@KUBECONFIG_PATH="$(shell pwd)/e2e/framework/maestro/.k3d-kubeconfig.yaml"; \
	echo "INFO: Checking for Kubeconfig at $$KUBECONFIG_PATH for integration tests..."; \
	if [ -f "$$KUBECONFIG_PATH" ] && [ -s "$$KUBECONFIG_PATH" ]; then \
		echo "INFO: Using kubeconfig from $$KUBECONFIG_PATH for integration tests..."; \
		KUBECONFIG="$$KUBECONFIG_PATH" go test -tags=integration -count=1 -timeout 20m -coverprofile=coverage.out -covermode=atomic $(OPTS) ./... ; \
	else \
		echo "ERROR: Kubeconfig $$KUBECONFIG_PATH not found or is empty. Please ensure 'make k3d/up' completed successfully before running integration tests."; \
		exit 1; \
	fi

.PHONY: run/runtime-integration-tests
run/runtime-integration-tests: k3d/up ## Execute runtime integration tests.
	@KUBECONFIG_PATH="$(shell pwd)/e2e/framework/maestro/.k3d-kubeconfig.yaml"; \
	echo "INFO: Checking for Kubeconfig at $$KUBECONFIG_PATH for runtime integration tests..."; \
	if [ -f "$$KUBECONFIG_PATH" ] && [ -s "$$KUBECONFIG_PATH" ]; then \
		echo "INFO: Using kubeconfig from $$KUBECONFIG_PATH for runtime integration tests..."; \
		KUBECONFIG="$$KUBECONFIG_PATH" go test -tags=integration -count=1 -timeout 20m ./internal/adapters/runtime/kubernetes/... ; \
	else \
		echo "ERROR: Kubeconfig $$KUBECONFIG_PATH not found or is empty. Please ensure 'make k3d/up' completed successfully before running runtime integration tests."; \
		exit 1; \
	fi

.PHONY: license-check
license-check: ## Execute license check.
	@go run github.com/google/addlicense -skip yaml -skip yml -skip proto -check .

.PHONY: run/e2e-tests
run/e2e-tests: k3d/up build setup/e2e deps/stop ## Execute end-to-end tests.
	@cd e2e; go test -count=1 -v $(OPTS) ./suites/...

#-------------------------------------------------------------------------------
#  Build and run
#-------------------------------------------------------------------------------

.PHONY: build
build: ## Build the project and generates a binary.
ifeq ($(shell go env GOARCH),amd64)
	@$(MAKE) build-linux-x86_64
	@ln -sf ./bin/maestro-linux-x86_64 ./bin/maestro
else ifeq ($(shell go env GOARCH),arm64)
	@$(MAKE) build-linux-arm64
	@ln -sf ./bin/maestro-linux-arm64 ./bin/maestro
else
	@echo "Unsupported architecture: $(shell go env GOARCH)"
	@exit 1
endif

.PHONY: build-linux-x86_64
build-linux-x86_64: ## Build the project and generates a binary for x86_64 architecture.
	@rm -f ./bin/maestro-linux-x86_64 || true
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ./bin/maestro-linux-x86_64 ./

.PHONY: build-linux-arm64
build-linux-arm64: ## Build the project and generates a binary for arm64 architecture.
	@rm -f ./bin/maestro-linux-arm64 || true
	@env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o ./bin/maestro-linux-arm64 ./

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
	@go run $(BUF) dep update && go run $(BUF) generate
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
	@$(DOCKER_COMPOSE) up -d --build
	@echo "Dependencies created successfully."

.PHONY: deps/start
deps/start: ## Start containers dependencies.
	@echo "Starting dependencies "
	@$(DOCKER_COMPOSE) start
	@echo "Dependencies started successfully."

.PHONY: deps/stop
deps/stop: ## Stop containers dependencies.
	@echo "Stopping dependencies "
	@$(DOCKER_COMPOSE) stop
	@echo "Dependencies stopped successfully."

.PHONY: deps/down
deps/down: ## Delete containers dependencies.
	@echo "Deleting dependencies "
	@$(DOCKER_COMPOSE) down
	@echo "Dependencies deleted successfully."


#-------------------------------------------------------------------------------
#  Easily start Maestro
#-------------------------------------------------------------------------------
.PHONY: maestro/start
maestro/start: k3d/up build ## Start Maestro with all of its dependencies.
	@echo "Starting maestro dependencies (postgres, redis)..."
	@cd ./e2e/framework/maestro; $(DOCKER_COMPOSE) up -d postgres redis
	@echo "Running database migrations..."
	@MAESTRO_MIGRATION_PATH="file://internal/service/migrations" go run main.go migrate;
	@echo "Starting maestro application services..."
	@cd ./e2e/framework/maestro; $(DOCKER_COMPOSE) up --build -d
	@echo "Maestro is up and running!"

.PHONY: maestro/down
maestro/down: k3d/down ## Delete Maestro and all of its dependencies.
	@echo "Deleting maestro..."
	@cd ./e2e/framework/maestro; $(DOCKER_COMPOSE) down
	@$(MAKE) k3d/down
	@echo "Maestro was deleted with success!"

#-------------------------------------------------------------------------------
#  Local Kubernetes cluster (k3d)
#-------------------------------------------------------------------------------

K3D_CLUSTER_NAME ?= maestro-dev

.PHONY: k3d/up
k3d/up: ## Create/Start the k3d cluster via docker-compose.
	@echo "INFO: Ensuring k3d service is up and cluster '$(K3D_CLUSTER_NAME)' is created via docker-compose..."
	@$(DOCKER_COMPOSE) up -d k3d
	@echo "INFO: Waiting for k3d cluster and kubeconfig file at ./e2e/framework/maestro/.k3d-kubeconfig.yaml..."
	@timeout=120; \
	interval=5; \
	elapsed=0; \
	until [ -f ./e2e/framework/maestro/.k3d-kubeconfig.yaml ] && [ -s ./e2e/framework/maestro/.k3d-kubeconfig.yaml ]; do \
		if [ $$elapsed -ge $$timeout ]; then \
			echo "ERROR: Timeout waiting for kubeconfig file."; \
			exit 1; \
		fi; \
		echo "INFO: Waiting for ./e2e/framework/maestro/.k3d-kubeconfig.yaml to be created and populated by k3d service (elapsed $$elapsed sec)..."; \
		sleep $$interval; \
		elapsed=$$((elapsed + interval)); \
	done
	@echo "INFO: k3d cluster '$(K3D_CLUSTER_NAME)' should be ready and kubeconfig exported."
	@echo "To use with kubectl, run: export KUBECONFIG=$(shell pwd)/e2e/framework/maestro/.k3d-kubeconfig.yaml"

.PHONY: k3d/down
k3d/down: ## Delete the local k3d cluster and stop the k3d docker-compose service.
	@echo "INFO: Attempting to delete k3d cluster '$(K3D_CLUSTER_NAME)' via k3d service (if running)..."
	@if $(DOCKER_COMPOSE) ps k3d | grep -q "k3d"; then \
		$(DOCKER_COMPOSE) exec k3d k3d cluster delete $(K3D_CLUSTER_NAME) || echo "INFO: k3d cluster delete command failed or cluster was not found."; \
	else \
		echo "INFO: k3d service not running, skipping cluster deletion command."; \
	fi
	@echo "INFO: Stopping and removing k3d service..."
	@$(DOCKER_COMPOSE) rm -sf k3d
	@echo "INFO: Removing k3d kubeconfig file..."
	@rm -f ./e2e/framework/maestro/.k3d-kubeconfig.yaml
	@echo "INFO: k3d cluster and service are down."
