SOURCES := $(shell \
	find . -not \( \( -name .git -o -name .go -o -name vendor \) -prune \) \
	-name '*.go')

.PHONY: deps
deps:
	@go get ./...

.PHONY: tools
tools:
	@go install ./...

################################################################################
## Lint and tests
################################################################################

.PHONY: goimports
goimports:
	@go run golang.org/x/tools/cmd/goimports -w $(SOURCES)

.PHONY: lint
lint:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run -E goimports ./...

.PHONY: run/unit-tests
run/unit-tests:
	@go test -count=1 -tags=unit -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: run/integration-tests
run/integration-tests:
	@go test -tags=integration -count=1 -timeout 20m -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: license-check
license-check:
	@go run github.com/google/addlicense -skip yaml -check .

################################################################################
## Build and run
################################################################################

.PHONY: build/worker
build/worker:
	@rm -f ./bin/worker
	@go build -o ./bin/worker ./cmd/worker

.PHONY: run/worker
run/worker: build/worker
	./bin/worker

.PHONY: build/management-api
build/management-api:
	@rm -f ./bin/management-api
	@go build -o ./bin/management-api ./cmd/management_api

.PHONY: run/management-api
run/management-api: build/management-api
	./bin/management-api

################################################################################
## Code generation
################################################################################

.PHONY: generate
generate:
	@go generate ./gen

.PHONY: wire
wire:
	@go run github.com/google/wire/cmd/wire ./...

.PHONY: mocks
mocks:
	@mockgen -source=internal/core/ports/port_allocator.go -destination=internal/adapters/port_allocator/mock/mock.go -package=mock
	@mockgen -source=internal/core/ports/runtime.go -destination=internal/adapters/runtime/mock/mock.go -package=mock
	@mockgen -source=internal/core/ports/room_storage.go -destination=internal/adapters/room_storage/mock/mock.go -package=mock
	@mockgen -source=internal/core/ports/instance_storage.go -destination=internal/adapters/instance_storage/mock/mock.go -package=mock
	@mockgen -source=internal/core/ports/operation_storage.go -destination=internal/adapters/operation_storage/mock/mock.go -package=mock
	@mockgen -source=internal/core/ports/scheduler_storage.go -destination=internal/adapters/scheduler_storage/mock/mock.go -package=mock
	@mockgen -source=internal/core/ports/operation_flow.go -destination=internal/adapters/operation_flow/mock/mock.go -package=mock
	@mockgen -source=internal/config/config.go -destination=internal/config/mock/mock.go -package=mock
	@mockgen -source=internal/core/operations/definition.go -destination=internal/core/operations/mock/definition.go -package=mock
	@mockgen -source=internal/core/operations/executor.go -destination=internal/core/operations/mock/executor.go -package=mock

################################################################################
## Migration and database make targets
################################################################################

.PHONY: migrate
migrate:
	@go run cmd/utils/utils.go migrate

################################################################################
## Local dependencies
################################################################################

.PHONY: deps/start
deps/start:
	@echo "Starting dependencies "
	@docker-compose --project-name maestro up -d
	@sleep 10
	@echo "Dependencies started successfully."

.PHONY: deps/stop
deps/stop:
	@echo "Stopping dependencies "
	@docker-compose --project-name maestro down
	@echo "Dependencies stoped successfully."
