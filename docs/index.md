<img align="right" width="300" height="260" src="./images/gopher-maestro.png">
# Overview
Maestro is a game room orchestration system. Each game room is a match execution context and a group of game rooms is organized in a Scheduler. Each time that a user requires some change in the Scheduler or game room, Maestro creates an Operation that is a change unit that will be enqueued and handled sequentially by a proper worker related exclusively to its respective Scheduler, in other words, each Scheduler has a worker that sequentialy handles Operations that is created to it.

A Scheduler defines the requirements of a game room as to how much memory and CPU it will need to execute and other information.

Each Operation has its own properties to be executed.

Maestro has five components: Management API, Game Rooms API, Execution Worker, Runtime Watcher and Metrics Reporter. They all deliver together the Maestro features that will be described in the next sections.

## Features & Components

Maestro is composed by:

- *Management API:* this is the component that the users will use to create your requests to interact with Maestro. For example: create a Scheduler, get Scheduler information, etc.
- *Execution Worker:* have an execution component to handle Operations to each Scheduler. For example: three Schedulers will have each one an execution component.
- *Game Rooms API*: game rooms API expose an HTTP API or a GRPc service to receive game rooms messages that symbolize their respective status. For example: when a game room is ready to receive matches it will send a message informing that.
- *Runtime Watcher:* is a component in Maestro that listen to Runtime events and reflect them in Maestro. For example: when a game room was created it is notified that this event happened.
- *Metrics Reporter:* time spaced this component query the Runtime for metrics related to rooms and expose them in an open metrics route. For example: how many occupied rooms and ready rooms are up.

## Support & Community

Include how users can reach out to you or your team. Include any details they should gather or have handy to facilitate resolution of the issue.
