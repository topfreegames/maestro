## Getting Started Guide

### Prerequisites

- Have a game room container image

### Learning Outcomes

After finishing this tutorial you will understand how:

- to set up your game room to communicate its health status with maestro
- to configure a new scheduler in maestro with a fixed number of replicas


#### Configuring your game room

For Maestro to be able to manage your game rooms, you need to ensure that your game room sends a periodic heartbeat to maestro.
This heartbeat is what we call `ping`, in which the room is able to inform its status (such as `ready` or `occupied`) to maestro.

For this, you can use [maestro-client](https://github.com/topfreegames/maestro-client) sdk, if you are using unity, or you can call
Maestro rooms API directly using two env vars that are configured in every game room managed by maestro by default.

`PUT scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping`
```json
{
    "status": "ready",
    "timestamp": "12312312313"
}
```

The **status** field can be: 
- `ready`: the room is ready to accept players.
- `occupied`: the room is occupied by one or more matches, and is not ready to accept more players.
- `terminating`: the room is terminating, and will not accept any new players.


#### Create a scheduler

Use the command below to create a new scheduler, this will make a POST request for `/schedulers` endpoint.

You need to change some parameters according to your game room image needs, for further details on all scheduler fields check
the [reference](../reference/Scheduler.md):

- **image**: your game room image.
- **game**: your game name (a same game can have multiple schedulers).
- **name**: your scheduler name (usually, the stack name).
- **spec.command**: any command that is required to run your game room.
- **spec.environment**: any environment variable required to run your game room.
- **spec.ports**: any port that must be exposed for clients to connect to the game room

```shell
curl --request POST \
  --url https://<maestro-url>/schedulers \
  --header 'Content-Type: application/json' \
  --header "Authorization: Basic <user:pass in base64>" \
  --data '{
	"name": "<your-scheduler-name-here>",
	"game": "<your-game-name-here>",
    "roomsReplicas": 1,
	"portRange": {
            "start": 20000,
            "end": 21000
	},
	"maxSurge": "10%",
	"spec": {
            "terminationGracePeriod": "100",
            "containers": [
                {
                    "name": "game-container",
                    "image": "<your-game-image-here>",
                    "imagePullPolicy": "IfNotPresent",
                    "command": [
                        <required-commands-for-game-image>
                        "sh example.sh"
                    ],
                    "environment": [
                        <required-commands-for-game-image>
                        {
                            "name": "EXAMPLE_NAME",
                            "value": "EXAMPLE_VALUE"
                        }
                    ],
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
            ]
	},
	"forwarders": []
}'
```

After running this command, a scheduler will be created with 1 game room replica.

Then you can use the following command to get the scheduler details such as how many rooms are ready or occupied:

```shell
curl --location --request GET "<maestro-url>/schedulers/info?game=<your-game-name-here>" \
 --header "Accept: application/json" \
 --header "Authorization: Basic <user:pass in base64>"
```


