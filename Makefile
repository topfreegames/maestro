SOURCES := $(shell \
	find . -not \( \( -name .git -o -name .go -o -name vendor -o -name '*.pb.go' -o -name '*.pb.gw.go' -o -name '*_gen.go' -o -name '*mock*' \) -prune \) \
	-name '*.go')

.PHONY: deps
deps:
	@go get ./...
	@go mod download

################################################################################
## Lint and tests
################################################################################

.PHONY: goimports
goimports:
	@go run golang.org/x/tools/cmd/goimports -w $(SOURCES)

.PHONY: lint
lint: lint/go lint/protobuf

.PHONY: lint/go
lint/go:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: lint/protobuf
lint/protobuf:
	@go run github.com/bufbuild/buf/cmd/buf lint --config buf.yaml

.PHONY: run/unit-tests
run/unit-tests:
	@go test -count=1 -tags=unit -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: run/integration-tests
run/integration-tests:
	@go test -tags=integration -count=1 -timeout 20m -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: license-check
license-check:
	@go run github.com/google/addlicense -skip yaml -skip yml -skip proto -check .

.PHONY: run/e2e-tests
run/e2e-tests:
	cd e2e; go mod download; go test -count=1 ./suites/...

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
