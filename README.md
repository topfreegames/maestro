# Maestro

```
Note

The version 9 of Maestro is under deprecation.
We will keep the support for this version after the v10 is released (more details below).
However, after that period, the version will no longer receive any updates.
```

Maestro is an unified system that automatically scales game rooms regardless of the protocol (TCP, UDP). This system is related to a matchmaker but does not handle the specificities of a match such as how many players fit in a room. It only deals with high level room occupation, i.e. is the room occupied or available.

## What does the Maestro do?

Maestro is a game room scheduler that is composed by a controller, a watcher, a worker, an API and a CLI.

## What problem is solved?

Maestro provides the ability for game teams to deploy Game Room Unities (GRUs) in an easy way. And the possibility to configure a more efficient allocation strategy to its GRUs. With that is expected that costs saved and less money lost with the game infrastructure.

## Recommended Integration Phase: Alpha

Usually, each game has its own time to include multiplayer feature, so generally is recommended to include it into the project during alpha.
If multuiplayer GRUs is required during prototyping, this module should be included in the prototyping phase as well.

## Dependencies
Maestro depends only infrastructure services to operate, the most required is a Kubernetes cluster where Maestro resides.

As internal dependecy Maestro needs a Redis and a Postgres database.

## How is the problem solved?

Maestro is composed by different modules that togheter deliver multiplayer feature. The following sessions list each part and a resume its functionality.

### Maestro Controller:

The Maestro Controller is responsible for managing the Game Room Unities (GRUs). It creates and gracefully terminates GRUs according to auto scaling policies defined by the user. It makes use of the Kubernetes cluster's API endpoints in order to have updated information about the GRUs managed by Maestro. It is also responsible for persisting relevant information in the database and managing rooms statuses.

### Maestro Watcher:

The Maestro Watcher ensures that at any time the Game Room Unities (GRUs) state is as expected. If the scaling policies say that one should have 10 GRUs of a given type, the Maestro Watcher will ask the Maestro Controller to create or terminate GRUs as needed. The desired state is kept in a database that is consulted by the Maestro Watcher (via Maestro Controller) each time it runs. It has a lock so Maestro can be scaled horizontally. Each scheduler (i.e. maestro scalable entity) has its own Maestro Watcher.

### Maestro Worker:

The Maestro Worker ensures that all valid schedulers (i.e. schedulers that exist in the database) have running watchers.

### Maestro Api:

The Maestro API is the connection of Maestro to the external world and with the game rooms itself. It is responsible for:

- Managing GRUs status and healthcheck (status are: creating, ready, occupied, terminating and terminated);
- Saving the scheduler config in a database that will be consulted by the Maestro Watcher;
- Managing the pool of GRUs with each GRU host IP and port;

### Maestro CLI:

The [Maestro CLI](https://github.com/topfreegames/maestro-cli) is a wrapper for the Maestro API endpoints. With this command installed on the manager machine it could interact and manage maestro schedulers, for example: list schedulers, describe a scheduler, shows last events happened.

### maestro-client:

The [maestro-client](https://github.com/topfreegames/maestro-client) is a lib for Unity and cocos2dx responsible for calling Maestri HTTP routes defined in the [room protocol](#room-protocol). It also must catch sigterm/sigkill and handle the room graceful shutdown.

## Additional information

*Documentation can be found in the [docs folder](./docs/README.md). This module is supported by the metagame module team*.

| Position | Name |
| --- | --- |
| Owner team | Game Services - Multiplayer |
| Documentation owner | [Arthur Nogueira](@arthur29) |
| Read the Docs | [latest](https://maestro.readthedocs.io/en/latest/) |
| Maestro CLI repo | [Maestro CLI](https://github.com/topfreegames/maestro-cli/tree/v1) |
| maesto-client repo | [maestro-client](https://github.com/topfreegames/maestro-client) |
