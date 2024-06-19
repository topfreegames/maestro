## Configuring Events Forwarding

### Prerequisites

- Have a game room container image that communicates with maestro through Maestro's rooms API

### Learning Outcomes

After finishing this tutorial you will understand how:

- to configure your room (ping) and player events to be forwarded to an external service (e.g. a matchmaking service)

### What is
Events forwarding is an optional feature in which every room event or player event is forwarded to an external service.

Through rooms API, Maestro provides [several endpoints](../reference/OpenAPI.md) for receiving events from the game rooms. These events
can be either room events (like room changing state from ready to occupied) or player events (like player joining or leaving the room).
Maestro rely only on room events for managing the game rooms, player events endpoint is designed to be used exclusively with the events forwarding feature,
since maestro does not depend on this information.

Usually Maestro is used with a Matchmaking service, and a matchmaking service generally will need to keep up-to-date with the pool of game rooms that are available or not.
Events forwarding feature exists for facilitating this integration, even being possible to make game rooms communicate with matchmaker directly.

### How to configure and enable events forwarder
To get events forwarding working in your scheduler, firstly you need to configure the events forwarder and enable it, this forwarder
configuration resides in the root of the scheduler structure itself.

[comment]: <> (YAML version)
<details>
    <summary>YAML version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
name: String
game: String
...
forwarders:
  - name: matchmaking
    enable: true
    type: gRPC
    address: 'external-matchmaker.svc.cluster.local:80'
    options:
      timeout: '1000'
      metadata:
        ...
        // Will vary according to the policy type.
        </pre>
    </div>
</details>


[comment]: <> (JSON version)
<details>
    <summary>JSON version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
{
  "name": "String",
  "game": "String",
  ...
  "forwarders": [
    {
      "name": "matchmaking",
      "enable": true,
      "type": "gRPC",
      "address": "external-matchmaker.svc.cluster.local:80",
      "options": {
        "timeout": "1000",
        "metadata": {
            ...
            // Will vary according to the user needs.
        } 
      }
    }
  ]
}
        </pre>
    </div>
</details>

- **name**: Name of the forwarder. Used only for reference (visibility and recognition);
- **enable**: Toggle to easily enable/disable the forwarder;
- **type**: Type of the forwarder. Right now, only accepts **gRPC**;
- **address**: Address used by the scheduler to forward events. E.g. 'api.example.com:8080';
- **options**: Optional parameters.
  - **timeout**: Timeout value for an event to successfully be forwarded;
  - **metadata**: Arbitrary metadata object that can contain any data that will be embedded in all event that is forwarded.
-------

## Events Forwarding Types
Currently, Maestro only supports gRPC forwarder type.

### GRPC
This event forwarding type uses the [GRPCForwarder service proto definition](https://github.com/topfreegames/protos/blob/master/maestro/grpc/protobuf/events.proto)
to forward events, this means that the external service should use gRPC protocol and implement this service to receive events.

#### Response
Maestro expects the forwarder event response to return a HTTP code, which is mapped internally to a gRPC code. This mapping is done in the [handlerGrpcClientResponse function](https://github.com/topfreegames/maestro/blob/main/internal/adapters/events/events_forwarder.go)

#### Client Configuration

The GRPC forwarder uses a client that will dial into the address of the forwarder configured in the scheduler and forward events to it. However, we need
to configure a KeepAlive mechanism so we constantly send HTTP/2 ping frames on the channel and detect broken connections. Without a KeepAlive mechanism,
broken TCP connections are only refreshed when Kernel kills the fd responsible due to inactivity, which can take up to 20 minutes.

Thus, there are some configurations that you can do to tweak the KeepAlive configuration. Those configs can be set either as an env var or in the `config.yaml`:

* `adapters.grpc.keepAlive.time`: After a duration of this time if the client doesn't see any activity it pings the server to see if the transport is still alive. If set below 10s, a minimum value of 10s will be used instead. Defaults to 30s.
* `adapters.grpc.keepAlive.timeout`: After having pinged for keepalive check, the client waits for a duration of Timeout and if no activity is seen even after that the connection is closed. Defaults to 5s.

[GRPC Official Doc Reference](https://grpc.io/docs/guides/keepalive/)

[GRPC Internals Reference](https://github.com/grpc/grpc/blob/master/doc/keepalive.md)
