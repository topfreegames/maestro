Overview
========

Maestro is a Kubernetes Game Room Scheduler.

## Goal

Have an unified system that automatically scales game rooms regardless of the protocol (TCP, UDP). This system is related to a matchmaker but does not handle the specificities of a match such as how many players fit in a room. It only deals with high level room occupation, i.e. is the room occupied or available. The rooms communicate directly with the matchmaker in order to register and unregister themselves from the matchmaking.

Let us define a Game Room Unity (GRU) as a Kubernetes service (type nodePort) associated with a single pod. This restriction is made because in AWS we cannot load balance UDP. We're using containerized applications and Kubernetes in order to simplify the management and scaling of the game rooms.

## Architecture

Maestro is a game room scheduler that is composed by a controller, a watcher, a worker, an API and a CLI. In the future we may have an UI for displaying metrics such as:

- % of rooms usage
- rate of room occupation increase/decrease
- rooms cpu and memory usage
- etc.

### maestro-controller

The controller is responsible for managing the Game Room Unities (GRUs). It creates and gracefully terminates GRUs according to auto scaling policies defined by the user. It makes use of the Kubernetes cluster's API endpoints in order to have updated information about the GRUs managed by Maestro. It is also responsible for persisting relevant information in the database and managing rooms statuses.

### maestro-watcher

The watcher ensures that at any time the Game Room Unities (GRUs) state is as expected. If the scaling policies say that one should have 10 GRUs of a given type, the watcher will ask the controller to create or terminate GRUs as needed. The desired state is kept in a database that is consulted by the watcher (via controller) each time it runs. It has a lock so Maestro can be scaled horizontally. Each scheduler (i.e. maestro scalable entity) has its own watcher.

### maestro-worker

The worker ensures that all valid schedulers (i.e. schedulers that exist in the database) have running watchers.

### maestro-api

The API is the connection of Maestro to the external world and with the game room itself. It is responsible for:

- Managing GRUs status and healthcheck (status are: creating, ready, occupied, terminating and terminated);
- Saving the scheduler config in a database that will be consulted by the watcher;
- Managing the pool of GRUs with each GRU host ip and port;

### maestro-cli

The CLI is a wrapper for the maestro-api endpoints.

### maestro-client

A client lib for Unity and cocos2dx responsible for calling maestro HTTP routes defined in the [room protocol](#room-protocol). It also must catch sigterm/sigkill and handle the room graceful shutdown.

## Configuring Maestro

The maestro binary receives a list of config files and spawn one maestro-controller for each config.

The config file must have the following information:

- Docker image
- Autoscaling policies
- Manifest yaml template
  1. Default configuration (ENV VARS)
  2. Ports and protocols (UDP, TCP)
  3. Resources requests (cpu and memory)


Example yaml config:

```yaml
name: pong-free-for-all     # this will be the name of the kubernetes namespace (it must be unique)
game: pong                  # several configs can refer to the same game
image: pong/pong:v123
affinity: node-affinity     # optional field: if set, rooms will be allocated preferentially to nodes with label "node-affinity": "true"
toleration: node-toleration # optional field: if set, rooms will also be allocated in nodes with this taint
occupiedTimeout: match-time # how much time a match has. If room stays with occupied status for longer than occupiedTimeout seconds, the room is deleted
ports:
  - containerPort: 5050     # port exposed in the container
    protocol: UDP           # supported protocols are TCP and UDP
    name: gamebinary        # name identifying the port (must be unique for a config)
  - containerPort: 8888
    protocol: TCP
    name: websocket
requests:                   # these will be the resources requests applied to the pods created in kubernetes
  memory: 1Gi               # they are used to calculate resource(cpu and memory) usage and trigger autoscaling when metrics triggers are defined
  cpu: 1000m                
limits:                     # these will be the resources limits applied to the pods created in kubernetes
  memory: "128Mi"           # they are used to decide how many rooms can run in each node
  cpu: "1"                  # more info: https://kubernetes.io/docs/tasks/configure-pod-container/assign-cpu-ram-container/
shutdownTimeout: 180        # duration in seconds the pod needs to terminate gracefully
autoscaling:
  min: 100                  # minimum amount of GRUs
  max: 1000                 # maximum amount of GRUs
  up:
    metricsTrigger:         # list of triggers that define the autoscaling behaviour
      - type: room          # the triggers can be of type room, cpu or memory
        threshold: 80       # percentage of the points that are above 'usage' needed to trigger scale up
        usage: 70           # minimum usage (percentage) that can trigger the scaling policy
        time: 600           # duration in seconds to wait before scaling policy takes place     
    cooldown: 300           # duration in seconds to wait before consecutive scaling
  down:
    metricsTrigger:
      - type: cpu
        threshold: 80       # percentage of the points that are above 'usage' needed to trigger scale down
        usage: 50           # maximum usage (percentage) the can trigger the scaling policy
        time: 900           # duration in seconds to wait before scaling policy takes place       
    cooldown: 300           # duration in seconds to wait before consecutive scaling
env:                        # environment variable to be passed on to the container
  - name: EXAMPLE_ENV_VAR
    value: examplevalue
  - name: ANOTHER_ENV_VAR
    value: anothervalue
cmd:                        # if the image can run with different arguments you can specify a cmd
  - "./room-binary"
  - "-serverType"
  - "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
forwarders:                # optional field: if set events will be forwarded for the grpc matchmaking plugin
  grpc:
    matchmaking:
      enabled: true
      metadata:            # the forwarder metadata is forwarded in scheduler events (create and update)
        matchmakingScript: default
        metadata:
          authTimeout: 10000
        minimumNumberOfPlayers: 1
        numberOfTeams: 1
        playersPerTeam: 6
        roomType: "10"
        tags:
          score: score
```

A JSON file equivalent to the yaml above can also be used.


## Room Protocol

Game rooms have four different statuses:

  - Creating

    From the time maestro starts creating the GRU in Kubernetes until a room ready is received.

  - Ready

    From the time room ready is called until a match started is received. It means the room is available for matches.

  - Occupied

    From the time match started is called until a match ended is received. It means the room is not available for matches.

  - Terminating

    From the time a sigkill/sigterm signal is received by the room until the GRU is no longer available in Kubernetes.

Maestro's auto scaling policies are based on the number of rooms that are in ready state.
