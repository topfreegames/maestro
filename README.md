<img align="right" width="300" height="260" src="docs/images/gopher-maestro.png">


*WARNING*: The [version v9.x](https://github.com/topfreegames/maestro/tree/v9) of Maestro is under deprecation, complete guide of the new version v10.x can be found [here](https://github.com/topfreegames/maestro/issues/283).

---
# Maestro 

[![Build Status](https://github.com/topfreegames/maestro/actions/workflows/test.yaml/badge.svg?branch=next)](https://github.com/topfreegames/maestro/actions/workflows/test.yaml)
[![Codecov Status](https://codecov.io/gh/topfreegames/maestro/branch/next/graph/badge.svg?token=KCN2SZDRJF)](https://codecov.io/gh/topfreegames/maestro)

[//]: # (One or two sentences description of the module/service in non technical terms and information about the owner. Text in quotes are examples.)

Game room management service, designed to manage multiple game room fleets in isolated schedulers.

## What does the module do?

[//]: # (A **non technical description** of the main tasks of the module in one or two paragraphs. After reading this section everybody should be able to understand which tasks are performed.)

Manage multiple game rooms fleets in Kubernetes clusters with user-created custom specifications (schedulers).

## What problem is solved?

[//]: # (A **non technical** explanation of what problem is solved by this module. This is **not** another description of what the module does, rather why the module exists in the first place.)

Maestro orchestrate game rooms fleets according to user specifications (schedulers), in which it can maintain a fixed number of game rooms up and running,
or can use autoscaling policies for costs optimization.

## Recommended Integration Phase: Alpha

[//]: # (In which phase is this module normally used for the prototyping, alpha, preproduction or production stage of game development.)

This module is not required for prototyping, but it is recommended to include it in the alpha phase so future development and deployment of new game room versions
can be better managed.

## Dependencies

[//]: # (List all the modules or other dependencies that module has, if possible with links to their repositories.)

Maestro does not have any dependencies, but it provides an events forwarding feature that can be used to integrate with external matchmaking services.

## How is the problem solved?

[//]: # (A **more technical description** how the module solved the problem description above. It should not get into too much detail, but provide enough information for a technical leader to understand the implications of using this module.)

```
Note
Currently, the only runtime that Maestro supports is Kubernetes.
```

With a scheduler, the user can define how a game room can be built and deployed on the runtime. Each scheduler
manages a fleet of game rooms in isolated namespaces.

Usually a scheduler will define which docker image will be used, its commands, resources that need to be allocated (cpu and memory), and other parameters for fleet management.

Every action that Maestro does on runtime to manage its resources is encapsulated in what we call an **Operation**. An Operation can be triggered by the user
or by Maestro itself. Maestro uses a queue to control operations flow and disposes of a worker that keeps processing operations.

Maestro provides two APIs for external and internal communication: 
- `management-api`: used by users to manage schedulers and operations. 
- `rooms-api`: used by game rooms to report its status back to Maestro.

## Additional information

[//]: # (Here a link to the complete module documentation as well as other additional information should be added as well as contact information.)

Documentation can be found in the [docs folder](./docs). This module is supported by the Wildlife's multiplayer team.

| Position            | Name               |
|---------------------|--------------------|
| Owner Team          | Multiplayer Team   |
| Documentation Owner | [Guilherme Carvalho](https://github.com/guilhermocc) |
