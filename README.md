<img align="right" width="300" height="260" src="docs/images/gopher-maestro.png">


*WARNING*: The [version v9.x](https://github.com/topfreegames/maestro/tree/v9) of Maestro is under deprecation, complete guide of the new version v10.x can be found [here](https://github.com/topfreegames/maestro/issues/283).

---
Maestro: Kubernetes Game Room Scheduler 
=======================================

[![Build Status](https://github.com/topfreegames/maestro/actions/workflows/test.yaml/badge.svg?branch=next)](https://github.com/topfreegames/maestro/actions/workflows/test.yaml)
[![Codecov Status](https://codecov.io/gh/topfreegames/maestro/branch/next/graph/badge.svg?token=KCN2SZDRJF)](https://codecov.io/gh/topfreegames/maestro)

## Docs
All documentation regarding this version (v10.x, AKA NEXT) can be accessed at https://topfreegames.github.io/maestro/.

## Dependencies

> **⚠ WARNING: Ensure using cgroupv1**
> 
> K3s needs to use the deprecated `cgroupv1`, to successfully run the project in your machine ensure that your current docker use this version.

### Grpc gateway
In order to run make generate with success, you need to have grpc-gateway dependencies installed with the following command:
```shell
go install \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
    google.golang.org/protobuf/cmd/protoc-gen-go \
    google.golang.org/grpc/cmd/protoc-gen-go-grpc
```

### Golang version
The project requires golang version 1.16 or higher.

## Building and running locally
1. Run `make setup` to get all required modules
2. Run `make generate` to generate mocks, protos and wire (dependency injection)
3. Run `make deps/up` to startup service dependencies
4. Run `make migrate` to migrate database with the most updated schema

### Management API Flavor
To start the management-api flavor locally, run:
```
make run/management-api
```

To test if the service (with dependencies) is up and running, try to create a scheduler by running the following command:
```
curl --location --request POST "localhost:8080/schedulers" \
--header "Content-Type: application/json" \
--header "Accept: application/json" \
--data-raw "{
	\"name\": \"scheduler-test\",
	\"game\": \"game-test\",
	\"state\": \"creating\",
	\"portRange\": {
		\"start\": 1,
		\"end\": 1000
	},
	\"maxSurge\": \"10%\",
	\"spec\": {
		\"terminationGracePeriod\": \"100\",
		\"containers\": [
			{
				\"name\": \"alpine\",
				\"image\": \"alpine\",
				\"imagePullPolicy\": \"IfNotPresent\",
				\"command\": [ \"sh\", \"-c\", \"while true; do sleep 1; done\" ],
				\"environment\": [
					{
						\"name\": \"env-var-name\",
						\"value\": \"env-var-value\"
					}
				],
				\"requests\": {
					\"memory\": \"100Mi\",
					\"cpu\": \"100m\"
				},
				\"limits\": {
					\"memory\": \"200Mi\",
					\"cpu\": \"200m\"
				},
				\"ports\": [
					{
						\"name\": \"port-name\",
						\"protocol\": \"tcp\",
						\"port\": 12345,
						\"hostPort\": 54321
					}
				]
			}
		],
		\"toleration\": \"\",
		\"affinity\": \"\"
	},
	\"forwarders\": [
		{
            \"name\": \"test-fwd\",
            \"enable\": true,
            \"type\": \"gRPC\",
            \"address\": \"www.NON-EXISTANT-URL.com\",
            \"options\": {
               \"timeout\": 1000,
               \"metadata\": {}
            }
		}
    ]
}"'
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

### Runtime watcher flavor
To start the runtime watcher flavor locally, run:
```
make run/runtime-watcher
```

## Running tests

1. Run `make run/unit-tests` to run all unit tests
2. Run `make run/integration-tests` to run all integration tests
3. Run `make run/e2e-tests` to run all E2E tests. NOTE: Currently it is not
   possible to run it with the development environment set. This command will
   stop the dev dependencies before running.
4. Run `make lint` to run all registered linters
