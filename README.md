Maestro: Kubernetes Game Room Scheduler
=======================================
[![Build Status](https://github.com/topfreegames/maestro/actions/workflows/test.yaml/badge.svg?branch=next)](https://github.com/topfreegames/maestro/actions/workflows/test.yaml)
[![Codecov Status](https://codecov.io/gh/topfreegames/maestro/branch/next/graph/badge.svg?token=KCN2SZDRJF)](https://codecov.io/gh/topfreegames/maestro)

## Building and running locally

1. Run `make deps` to get all required modules
2. Run `make generate` to generate mocks, protos and wire (dependency injection)
3. Run `make deps/start` to startup service dependencies
4. Run `make migrate` to migrate database with the most updated schema

### Management API Flavor
To start the management-api flavor locally, run:
```
make run/management-api
```

To test if the service (with dependencies) is up and running, try to create a scheduler by running the following command:
```
curl --location --request POST 'http://localhost:8080/schedulers' \
--header 'Content-Type: application/json' \
--data-raw '{
    "name": "zooba-blue",
    "game": "zooba",
    "version": "1.0.0"
}'
```

### Worker Flavor
To start the worker flavor locally, run:
```
make run/worker
```

If you've create a scheduler following the last steps of `management-api` test, starting the worker will execute the `create_scheduler` operation, you can check if the operation is executed by executing the following command:
```
export KUBECONFIG=$(pwd)/kubeconfig.yaml
kubectl get namespaces | grep zooba
```

## Running tests

1. Run `make run/unit-tests` to run all unit tests
2. Run `make run/integration-tests` to run all integration tests
3. Run `make run/e2e-tests` to run all E2E tests. NOTE: currently it is not
   possible to run it with the development environment set, so before running it
   perform a `make deps/stop` and stop all the components running.
4. Run `make lint` to run all registered linters
