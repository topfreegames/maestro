Maestro: Kubernetes Game Room Scheduler
=======================================

## Load Test:

### UDP

#### Spec:

- 900 GRUs
- 8 players per game room (results in 7200 CCU)
- 20 messages/second IN and OUT per player (144000 messages/second IN and OUT per second, 4800 messages/second per node)

Traffic Source Generator:

- Program that opens UDP sockets
- Sends messages to game room (20 messages/second)
- Listens to the messages sent by the game room
- Calculates how many messages/second are sent and received (aggregates all the sockets) --> used to estimate data loss

Game Room simulator:

- Receives UDP packets from the clients (Traffic Source Generator)
- Each time a new client sends a packet it register the client in a clients pool
- Sends messages to the registered clients (20 messages/second)
- Calculates how many messages/second are sent and received (aggregates all the sockets) --> used to estimate data loss

#### Results:

We ran several simulations with the following config:
  - Kubernetes 1.6 with kube-net
  - UDP Echo Server:
    - 30 nodes m4.large
    - 900 GRUs
  - UDP Client:
    - 4 AWS instances
    - 1 to 16 processes per instance
    - 10 to 40 msgs/s

Observations:
  - Nodes CPU usage
    - In the nodes the load average was about 1.5
    - kube-proxy CPU ranged from 1% to 4%
    - From time to time there was a peak in iptables restore (up to 20% CPU) consistent with calls to the syncProxyRules method
  - Master CPU usage
    - No impact was visible in the master CPU usage.
  - Kube-System resources usage:
    - CPU usage ranged from 1.6 to 2.2 Cores
    - Memory usage stayed around 2.5Gb
  - What happens when a new service is created:
    - No impact was visible in the application performance or in the metrics above
    - There was a peak of iptables CPU usage also consistent with the syncProxyRules method

We also ran two different tests:
  1. Calling the pod's node ip directly
  2. Calling a random node ip

We observed that when we called a random node ip the metrics reported some data loss, this is probably due to the higher latency caused by routing the message to the pod's node. Since we're not waiting for all responses to come back before printing the stats, it showed as if messages were being lost: received/sent ~ 96% instead of 99.8% when calling the pod's node ip directly.

Another relevant observation when we call a random ip, kube-net allocates 2 ports for each hop it has to make until reaching the pod's node. When calling the pod's node ip no additional ports were allocated.

### TCP

To be done later.
