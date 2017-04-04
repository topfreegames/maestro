Maestro: Kubernetes Game Room Scheduler
=======================================

## Goal:

Have an unified system that automatically scales game rooms regardless of the protocol (TCP, UDP, HTTP). This system is related to a matchmaker but does not handle the specificities of a match such as how many players fit in a room. It only deals with high level room occupation, i.e. is the room occupied or available. The rooms communicate directly with the matchmaker in order to register and unregister themselves from the matchmaking.

Let us define a Game Room Unity (GRU) as a Kubernetes service (type nodePort) associated with a single pod. This restriction is made because in AWS we cannot load balance UDP. We're using containerized applications and Kubernetes in order to simplify the management and scaling of the game rooms.

## Architecture:

Create a room scheduler that is composed by a controller, a watcher, an API and a CLI. In the future we may have an UI for displaying metrics such as:

- % of rooms usage
- rate of room occupation increase/decrease
- rooms cpu and memory usage
- etc.


### maestro-controller:

The controller is responsible for managing the Game Room Unities (GRUs). It creates, gracefully restarts and gracefully terminates GRUs according to auto scaling policies defined by the user. It makes use of the Kubernetes cluster's API endpoints in order to have updated information about the GRUs managed by the scheduler.

### maestro-watcher:

The watcher ensures that at any time, the Game Room Unities (GRUs) state is as expected. If the scaling policies say that one should have 10 GRUs of a given type, the watcher will ask the controller to create or remove GRUs as needed. The desired state is kept in a database that is consulted by the watcher each time it runs. It has a lock so maestro can be scaled horizontally.

### maestro-api:

The API is the connections of maestro to the external world. It is responsible for:

- Managing GRUs status and healthcheck (status are: creating, ready, occupied, restarting, terminating);
- Saving the scheduler config in a database that will be consulted by the watcher;
- Managing the pool of GRUs with each GRU ip and port;

### maestro-cli:

The CLI is an abstraction on top of the maestro-api.

## Configuring Maestro

The maestro binary receives a list of config files and spawn one maestro-controller for each config.

The config file must have the following information:

- Docker image
- Autoscaling policies
- Manifest yaml template
  1. Default configuration (ENV VARS)
  2. Ports and protocols (UDP, TCP)
  3. Resources requests (cpu and memory)

## TODOs:

- [ ] Define Architecture
  - [ ] Validate Kubernetes performance with a large amount of services
- [ ] Formalize room protocol
- [ ] Release map

## Release Map:

TBD

## Doubts

- Can Kubernetes handle thousands of services?
- How to manage different versions running at the same time? Will the matchmaker be responsible for it?
- How to properly tune autoscaling policies?

## Architecture Validation and Tests

### Validating Kubernetes performance

Initial setup: Kubernetes 1.5

Testing with 30 nodes m4.large and 900 GRUs (pods + service) using a simple image for UDP listener: [mendhak/udp-listener:latest](https://hub.docker.com/r/mendhak/udp-listener/).

To be checked:

  - [ ] Nodes CPU usage
  - [ ] Master CPU usage
  - [ ] Kube-System resources usage
  - [ ] Kube-Proxy logs
  - [ ] Load test 
    - [ ] What happens when a new service is created

#### Observations:

- While running the 900 pods + services
  - kube-system used 30 cores (CPU) and 9Gi (memory usage). Each kube-proxy pod consumes about 1 core.
  - syncProxyRules took 16.810688ms

- Without any test pods
  - kube-system used 1 core (CPU) and 7Gi (memory usage). Each kube-proxy pod consumes about 0.02 core.
  - syncProxyRules took 966.091651ms (note: it runs by default every 1000ms)

Changing --iptables-min-sync-period to 10s seems to have improved the CPU usage performance, but the cost is that anytime a new service is created it can take up to 10s until they are available.

[This Kubernetes PR](https://github.com/kubernetes/kubernetes/pull/38996) might be related to the services scaling problem and it is available in Kubernetes 1.6 and greater.
