Operation is a core concept at Maestro, and it represents executions done in multiple layers of Maestro, a state update, or a configuration change. Requests sent from game rooms are not handled in operations. Instead, the services directly take those. In most scenarios, those will be only state updates and won't perform any action on the scheduler or rooms.

## Definition and executors

Maestro will have multiple operations, and those will be set using pairs of definitions and executors. An operation definition consists of the operation parameters. An operation executor is where the actual operation logic is implemented, and it will receive as input its correlated definition. So, for example, the `CreateSchedulerExecutor` will always receive a `CreateSchedulerDefinition`.

```
┌─────────────────────────────────┐
│ Operations                      │
│ ┌─────────────────────────────┐ │
│ │ Operation implementation    │ │
│ │                             │ │
│ │ ┌──────────┐   ┌──────────┐ │ │
│ │ │Definition│   │Executor  │ │ │
│ │ └──────────┘   └──────────┘ │ │
│ │                             │ │
│ └─────────────────────────────┘ │
│                                 │
│ ┌─────────────────────────────┐ │
│ │ Operation implementation    │ │
│ │                             │ │
│ │ ┌──────────┐   ┌──────────┐ │ │
│ │ │Definition│   │Executor  │ │ │
│ │ └──────────┘   └──────────┘ │ │
│ │                             │ │
│ └─────────────────────────────┘ │
│                                 │
│ ┌─────────────────────────────┐ │
│ │ Operation implementation    │ │
│ │                             │ │
│ │ ┌──────────┐   ┌──────────┐ │ │
│ │ │Definition│   │Executor  │ │ │
│ │ └──────────┘   └──────────┘ │ │
│ │                             │ │
│ └─────────────────────────────┘ │
│                                 │
└─────────────────────────────────┘
```

## Lifecycle
```
                     ┌────────────────────┐
                     │Create new operation│
                     └──────────┬─────────┘
                                │
                                │
                         ┌──────▼──────┐
                         │Pending      │
                         └──────┬──────┘
                                │
                                │
                 ┌──────────────▼────────────────┐
                 │Will the operation be executed?│
                 └──────┬─────────────────┬──────┘
                        │Yes              │No
                        │                 │
                 ┌──────▼──────┐   ┌──────▼──────┐
                 │In progress  │   │Evicted      │
                 └──────┬──────┘   └─────────────┘
        Succeeds        │
        ┌───────────────┤
        │               │Fails
        │               │
 ┌──────▼──────┐ ┌──────▼──────┐
 │Finished     │ │ Error       │
 └─────────────┘ └─────────────┘
```
