## Development

-----

### Setting up the environment 

#### Grpc gateway
In order to run make generate with success, you need to have grpc-gateway dependencies installed with the following command:
```shell
go install \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
    google.golang.org/protobuf/cmd/protoc-gen-go \
    google.golang.org/grpc/cmd/protoc-gen-go-grpc
```

#### Golang version
The project requires golang version 1.20 or higher.

#### Building and running
1. Run `make setup` to get all required modules
2. Run `make generate` to generate mocks, protos and wire (dependency injection)
3. Run `make deps/up` to startup service dependencies
4. Run `make migrate` to migrate database with the most updated schema

#### Running tests

1. Run `make run/unit-tests` to run all unit tests
2. Run `make run/integration-tests` to run all integration tests
3. Run `make run/e2e-tests` to run all E2E tests. NOTE: Currently it is not
   possible to run it with the development environment set. This command will
   stop the dev dependencies before running.
4. Run `make lint` to run all registered linters

---

### Running locally

To help you get along with Maestro, by the end of this section you should have a scheduler up and running.

#### Prerequisites
- Golang v1.20+
- Linux/MacOS environment
- Docker

#### Clone Repository
Clone the [repository](https://github.com/topfreegames/maestro) to your favorite folder.

#### Getting Maestro up and running
> For this step, you need docker running on your machine.

> **WARNING: Ensure using cgroupv1**
>
> K3s needs to use the deprecated `cgroupv1`, to successfully run the project in your machine ensure that your current docker use this version.


In the folder where the project was cloned, simply run:

```shell
make maestro/start
```

This will build and start all containers needed by Maestro, such as databases and maestro-modules. This will also start
all maestro components, including rooms api, management api, runtime watcher, and execution worker.

Because of that, be aware that it might take some time to finish.

#### Find rooms-api address
To simulate a game room, it's important to find the address of running **rooms-api** on the local network.

To do that, with Maestro containers running, simply use:

```shell
docker inspect -f '{{range.NetworkSettings.Networks}}{{.Gateway}}{{end}}' {{ROOMS_API_CONTAINER_NAME}}
```

This command should give you an IP address.
This IP is important because the game rooms will use it to communicate their status.

#### Create a scheduler
If everything is working as expected now, each Maestro-module is up and running.
Use the command below to create a new scheduler:

> Be aware to change the {{ROOMS_API_ADDRESS}} for the one found above.
```shell
curl --request POST \
  --url http://localhost:8080/schedulers \
  --header 'Content-Type: application/json' \
  --data '{
	"name": "scheduler-run-local",
	"game": "game-test",
	"state": "creating",
	"portRange": {
		"start": 1,
		"end": 1000
	},
	"maxSurge": "10%",
	"spec": {
		"terminationGracePeriod": "100s",
		"containers": [
			{
				"name": "alpine",
				"image": "alpine",
				"imagePullPolicy": "IfNotPresent",
				"command": [
					"sh",
					"-c",
					"apk add curl && while true; do curl --request PUT {{ROOMS_API_ADDRESS}}:8070/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping --data-raw '\''{\"status\": \"ready\",\"timestamp\": \"12312312313\"}'\'' && sleep 5; done"
				],
				"environment": [],
				"requests": {
					"memory": "100Mi",
					"cpu": "100m"
				},
				"limits": {
					"memory": "200Mi",
					"cpu": "200m"
				},
				"ports": [
					{
						"name": "port-name",
						"protocol": "tcp",
						"port": 12345
					}
				]
			}
		],
		"toleration": "",
		"affinity": ""
	},
	"forwarders": []
}'
```

#### Congratulations
If you followed the steps above you have Maestro running in your local machine, and with a [scheduler](../reference/Scheduler.md) to try different [operations](../reference/Operations.md) on it.
Feel free to explore the available endpoints in the [API](../reference/OpenAPI.md) hitting directly the management-API.

If you have any doubts or feedbacks regarding this process, feel free to reach out in [Maestro's GitHub repository](https://github.com/topfreegames/maestro) and open an issue/question.

## Local Environment Setup

To run Maestro locally for development or testing, you'll need the following tools installed:

*   **Go (version 1.23 or higher)**
*   **Docker**
*   **Docker Compose**
*   **kubectl (Kubernetes CLI)** - *Optional, for interacting with the k3d cluster directly.*
*   **Make**

(Note: `k3d` itself is now managed via Docker Compose as part of the local dependencies, so direct installation of `k3d` CLI is no longer a prerequisite for basic `make` target usage.)

### Running Maestro Locally with k3d (Managed by Docker Compose)

1.  **Set up dependencies (Postgres, Redis & k3d Kubernetes cluster):**
    The `docker-compose.yaml` file at the root of the project defines services for Postgres, Redis, and `k3d`. The `k3d` service will automatically create a Kubernetes cluster named `maestro-dev` and export its kubeconfig to `e2e/framework/maestro/.k3d-kubeconfig.yaml`.

    To start these dependencies, including the k3d cluster:
    ```bash
    make deps/up
    ```
    Alternatively, to only start/ensure the k3d cluster is up:
    ```bash
    make k3d/up
    ```
    This command will also wait until the kubeconfig file is ready.
    The `maestro-dev` cluster will have port `38080` on your host mapped to the cluster's ingress (port 80 on the load balancer).

2.  **Start Maestro Application Services:**
    Once the k3d cluster is running and its kubeconfig is available (handled by `make k3d/up`), you can start the Maestro application services (management-api, rooms-api, etc.). These are defined in `e2e/framework/maestro/docker-compose.yml`.

    The recommended way to start everything, including dependencies and Maestro services, is:
    ```bash
    make maestro/start
    ```
    This target handles the correct order of operations: starts k3d, ensures kubeconfig is present, builds Maestro, runs migrations, and then starts the Maestro services using the `e2e/framework/maestro/docker-compose.yml` file. The Maestro services in this compose file are configured to use the kubeconfig generated by the `k3d` service.

    If you need to start them manually after `make k3d/up` and `make build`:
    ```bash
    # Ensure migrations are run if it's the first time or after schema changes
    # MAESTRO_MIGRATION_PATH="file://internal/service/migrations" go run main.go migrate
    # (The make maestro/start target handles migrations)

    docker-compose -f e2e/framework/maestro/docker-compose.yml up -d management-api rooms-api worker runtime-watcher # Add other services as needed
    ```

3.  **Build and Run Maestro components (alternative to full Docker Compose setup for services):**
    If you prefer to run Maestro components (e.g., `management-api`, `worker`) directly on your host for development (after `make deps/up` has started Postgres, Redis, and k3d):
    ```bash
    make build
    ```
    Then run individual components, e.g.:
    ```bash
    # Ensure KUBECONFIG points to the k3d cluster
    export KUBECONFIG="$(pwd)/e2e/framework/maestro/.k3d-kubeconfig.yaml"
    MAESTRO_INTERNALAPI_PORT=8081 MAESTRO_API_PORT=8080 go run main.go start management-api -l development
    ```
    These components will connect to Postgres and Redis (from the root `docker-compose`) and the k3d Kubernetes cluster (using the exported KUBECONFIG).

4.  **Running Tests:**
    *   **Unit Tests:** `make run/unit-tests`
    *   **Integration Tests:** `make run/integration-tests` (These will run against the k3d cluster managed by Docker Compose)
    *   **E2E Tests:** `make run/e2e-tests`

5.  **Stopping the local k3d cluster and dependencies:**
    To delete the `maestro-dev` k3d cluster and remove its Docker Compose service:
    ```bash
    make k3d/down
    ```
    To stop all dependencies defined in the root `docker-compose.yaml` (Postgres, Redis, k3d):
    ```bash
    make deps/down
    ```
    To stop the Maestro application services defined in `e2e/framework/maestro/docker-compose.yml`:
    ```bash
    docker-compose -f e2e/framework/maestro/docker-compose.yml down
    ```
    The `make maestro/down` target handles stopping both the k3d cluster and the Maestro application services.

### CI Environment

Note: In our CI environment (GitHub Actions), we use **KinD (Kubernetes in Docker)** to run integration and runtime-integration tests. This provides a fresh Kubernetes cluster for each test run.

## Project overview

