To help you get along with Maestro, by the end of this section you should have a scheduler up and running.

## Prerequisites
- Golang v1.17+
- Linux/MacOS environment
- Docker

### Clone Repository
Clone the [repository](https://github.com/topfreegames/maestro) to your favorite folder.

## Building and running locally
> For this step, you need docker running on your machine.

In the folder where the project was cloned, simply run:

```shell
make maestro/start
```

This will build and start all containers needed by Maestro, such as databases and maestro-modules.
Because of that, be aware that it might take some time to finish.

## Find rooms-api address
To simulate a game room, it's important to find the address of running **rooms-api** on the local network.

To do that, with Maestro containers running, simply use:

```shell
docker inspect -f '{{range.NetworkSettings.Networks}}{{.Gateway}}{{end}}' {{ROOMS_API_CONTAINER_NAME}}
```

This command should give you an IP address.
This IP is important because the game rooms will use it to communicate their status.

## Create a scheduler
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
		"terminationGracePeriod": "100",
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

## Congratulations
If you followed the steps above you have Maestro running in your local machine, and with a [scheduler](../reference/Scheduler.md) to try different [operations](../reference/Operations.md) on it.
Feel free to explore the available endpoints in the [API](../reference/OpenAPI.md) hitting directly the management-API.

If you have any doubts or feedbacks regarding this process, feel free to reach out in [Maestro's GitHub repository](https://github.com/topfreegames/maestro) and open an issue/question.