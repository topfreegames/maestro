Maestro v10 (aka Next) is a complete rewrite of the service, focusing on improving code maintainability, knowing what is being done on the rooms/schedulers, and changing the state management.

The main focus of the refactor was to take Maestro's strengths and areas that could be improved. After brainstorming, we've decided that a simpler and more concrete model was necessary to maximize those areas.

The proposed model is very similar to what continuous integration services do. You have a definition of what has to be done, and there is an implementation that takes it and executes it. This describes the overall model of this version. We call it "Operations".


## Architecture

Besides the new model described above, this release presents a completely different architecture and code organization.

```
                        │
Current components (v9) │ Next components (v10)
                        │
┌──────┐ ┌───┐          │ ┌─────────────────┐    ┌─────────┐
│Worker│ │API│          │ │Operations Worker│    │Rooms API│
└──────┘ └───┘          │ └─────────────────┘    └─────────┘
                        │
                        │ ┌──────────────┐ ┌───────────────┐
                        │ │Management API│ │Runtime Watcher│
                        │ └──────────────┘ └───────────────┘
```

-   Management API (Public API): This is the API that manages the scheduler and rooms. It is the component the users (usually game engineers) use while interacting with Maestro, making updates, and fetching scheduler information;
-   Rooms API (Internal API): API used by the game rooms to report their status to Maestro and communicate any extra information that may apply;
-   Operations executor worker: Worker that takes queued operations and processes them;
-   Runtime watcher: Watches any state change that happens on the Runtime and takes the necessary action (like updating some Maestro state or triggering an operation);

## Code structure
Maestro codebase is organized following some concepts of Hexagonal architecture (Ports & Adapters). This "style" was used as a base of the development and had some adaptations. For example, the API is not considered a primary/input Port. Instead, it is outside the core package.

Most of the business logic is present at services, and they rely on the ports to not depend on the specific implementations.

Entities are the structs that hold data on the system, and they don't provide any logic besides its initialization and validation. Therefore, services and ports should "communicate" using entities so that the entire system "speaks" the same language regarding data structs.

The API is responsible for converting the external protocol (currently gRPC with grpc-gateway) to entities, invoking the necessary service, and converting the response back.

### Workers
Maestro now provides a form of extending and adding more workers if they follow a worker interface. This was done to have the same pattern applied to the operation execution worker and runtime watcher. Workers are managed by the "WorkersManager," which starts an instance for each scheduler present on the storage. With this model, the implementation doesn't have to deal with multiple tenants (schedulers).

Workers are not inside the service package, although they hold business logic about execution and flows. They were isolated since workers are a kind of service that has to implement an interface. Besides this difference, workers should be treated like core services.
