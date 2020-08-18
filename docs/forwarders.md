Event Forwarders
========

Event forwarders are pluggable components that forwards events like: RoomReady, RoomTerminated, RoomTerminating, etc... to other services.

A event forwarder is a go native plugin that should be compiled and put into bin folder, it should contain a method ```func NewForwarder(config *viper.Viper) (eventforwarder.EventForwarder)``` that returns a configured instance of a struct that implements "eventforwarder.EventForwarder".

An example is provided in the plugins/grpc folder, compile it with:
```
go build -o bin/grpc.so -buildmode=plugin plugins/grpc/forwarder.go
```

## Configuration
Then to turn it on, include a config like that in the active config file:
```
forwarders:
  grpc:
    matchmaking:
      address: "10.0.23.57:10000"
    local:
      address: "localhost:10000"
```

In this example, maestro will look for a plugin "grpc.so" in the bin folder and create 2 forwarders from it, matchmaking and local one, each using a different address.
Then, every time a room is changing states, all forwarders will be called with infos about the change.

### Testing
There's also a route: ```/scheduler/{schedulerName}/rooms/{roomName}/playerevent``` that can be called like that, for example:
```
curl -X POST -d '{"timestamp":12424124234, "event":"playerJoin", "metadata":{"playerId":"sime"}}' localhost:8080/scheduler/some/rooms/r1/playerevent
```
It will forward the playerEvent "playerJoin" with the provided metadata and roomId to all the configured forwarders.

For the provided plugin, the valid values for event field are: ['playerJoin','playerLeft']

## Responses
All forwarders responses are put in consideration even if they're not returned
to the clients. This means that if a forwarder fails the API notifying the event
also fails. This behaviour is deprecated and in future versions the forwarder
failure/success won't impact any of the notifiers. To achieve this in the
current version you can add the `forwardMessage (default: true)` configuration
to your scheduler like in this example:
```
forwarders:
  grpc:
    matchamaking:
      enabled: true
      forwardResponse: false
```
