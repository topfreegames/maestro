Maestro: Kubernetes Game Room Scheduler
=======================================
[![Build Status](https://github.com/topfreegames/maestro/actions/workflows/test.yaml/badge.svg?branch=next)](https://github.com/topfreegames/maestro/actions/workflows/test.yaml)
[![Codecov Status](https://codecov.io/gh/topfreegames/maestro/branch/next/graph/badge.svg?token=KCN2SZDRJF)](https://codecov.io/gh/topfreegames/maestro)

## Building and running locally

1. Run `make get` to get all required modules
2. Run `make generate` to generate code from protos
2. Run `make wire` to generate code from wire dependency injector
3. Run `make deps/start` to startup service dependencies
4. Run `make migrate` to migrate database with the most updated schema

### To start worker
```
make run/worker
```

### To start the management-api
```
make run/management-api
```

## Running tests

1. Run `make run/unit-tests` to run all unit tests
2. Run `make run/integration-tests` to run all integration tests
3. Run `make lint` to run all registered linters
