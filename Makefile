SOURCES := $(shell \
	find . -not \( \( -name .git -o -name .go -o -name vendor \) -prune \) \
	-name '*.go')

.PHONY: lint
lint:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run -E goimports ./...

.PHONY: run-unit-tests
run-unit-tests:
	@go test -count=1 -tags=unit -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: run-integration-tests
run-integration-tests:
	@go test -tags=integration -count=1 -timeout 20m -coverprofile=coverage.out -covermode=atomic ./...

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

.PHONY: goimports
goimports:
	@go run golang.org/x/tools/cmd/goimports -w $(SOURCES)

.PHONY: wire
wire:
	@go run github.com/google/wire/cmd/wire ./api/... ./cmd/... ./config/... ./internal/... ./manifests/... ./tools/...

.PHONY: run-worker
run-worker:
	@go run cmd/worker/wire_gen.go cmd/worker/worker.go

.PHONY: run-management-api
run-management-api:
	@go run cmd/management_api/wire_gen.go cmd/management_api/management_api.go

.PHONY: generate
generate: buf/install
	@go generate ./gen

.PHONY: buf/install
buf/install:
	@go install \
	    google.golang.org/protobuf/cmd/protoc-gen-go \
	    google.golang.org/grpc/cmd/protoc-gen-go-grpc \
	    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
	    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
	    github.com/bufbuild/buf/cmd/buf
