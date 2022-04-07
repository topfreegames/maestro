Table of Contents
---

- [What Is](#what-is)
  - [Definition and Executors](#definition-and-executors)
- [Operation Structure](#operation-structure)
  - [Input](#input)
  - [Execution History](#execution-history)
- [How does Maestro handle operations](#how-does-maestro-handle-operations)
- [States](#state)
  - [State Machine](#state-machine)
- [Lifecycle](#lifecycle)
- [Lease](#lease)
  - [What is](#what-is-the-operation-lease)
  - [Why operations have it](#why-operations-have-it)
  - [Troubleshooting operations with lease](#troubleshooting)
  - [Operation Lease Lifecycle](#operation-lease-lifecycle)
- [Available Operations](#available-operations)
  - [Create Scheduler](#create-scheduler)
  - [Create New Scheduler Version](#create-new-scheduler-version)
  - [Switch Active Version](#switch-active-version)
  - [Add Rooms](#add-rooms)
  - [Remove Rooms](#remove-rooms)


## What is
Operation is a core concept at Maestro, and it represents executions done in multiple layers of Maestro, a state update, or a configuration change.
Operations can be created by user actions while managing schedulers (e.g. consuming management API), or internally by Maestro to fulfill internal states requirements

Operations are heavily inspired by the [Command Design Pattern](https://en.wikipedia.org/wiki/Command_pattern)

### Definition and executors
Maestro will have multiple operations, and those will be set using pairs of definitions and executors.
An operation definition consists of the operation parameters.
An operation executor is where the actual operation execution and rollback logic is implemented, and it will receive as input its correlated definition.
So, for example, the `CreateSchedulerExecutor` will always receive a `CreateSchedulerDefinition`.

```mermaid
flowchart TD
  subgraph operations [Operations]
    subgraph operation_implementation [Operation Impl.]
      definition(Definition)
      executor(Executor)
    end
    subgraph operation_implementation2 [Operation Impl.]
      definition2(Definition)
      executor2(Executor)
    end
    subgraph operation_implementation3 [Operation Impl.]
      definition3(Definition)
      executor3(Executor)
    end
    ...
  end
```

## Operation Structure
- **id**: Unique operation identification. Auto-Generated;
- **status**: Operations status. For reference, see [here](#state).
- **definitionName**: Name of the operation. For reference, see [here](#available-operations).
- **schedulerName**: Name of the scheduler which this operation affects.
- **createdAt**: Timestamp representing when the operation was enqueued.
- **input**: Contains the input value for this operation. Each operation has its own input format.
  For details, see [below](#input).
- **executionHistory**: Contains logs with detailed info about the operation execution. See [below](#execution-history).

```yaml
id: String
status: String
definitionName: String
schedulerName: String
createdAt: Timestamp
input: Any
executionHistory: ExecutionHistory
```

### Input
- Create Scheduler
```yaml
scheduler: Scheduler
```
- Create New Scheduler Version
```yaml
scheduler: Scheduler
```
- Switch Scheduler Version
```yaml
newActiveVersion: Scheduler
```
- Add Rooms
```yaml
amount: Integer
```
- Remove Rooms
```yaml
amount: Integer
```

### Execution History
```yaml
createdAt: timestamp
event: String
```
- **createdAt**: When did the event happened.
- **event**: What happened. E.g. "Operation failed because...".

## How does Maestro handle operations
- Each scheduler has 1 operation execution (no operations running in parallel for a scheduler).
- Every operation execution has 1 queue for pending operations.
- When the worker is ready to work on a new operation, it'll pop from the queue.
- The operation is executed by the worker following the lifecycle described [here](#lifecycle).

## State
An operation can have one of the Status below:

- **Pending**: When an operation is enqueued to be executed;

- **Evicted**: When an operation is unknown or should not be executed By Maestro;

- **In Progress**: Operation is currently being executed;

- **Finished**: Operation finished; Execution succeeded;

- **Error**: Operation finished. Execution failed;

- **Canceled**: Operation was canceled by the user.

### State Machine
<!DOCTYPE html>
<html lang="en">
  <head>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/mermaid/8.14.0/mermaid.min.js"></script>
  </head>
  <body>
    <div class="mermaid">
      flowchart TD
        pending(Pending)
        in_progress(In Progress)
        evicted(Evicted)
        finished(Finished)
        canceled(Canceled)
        error(Error)
        pending --> in_progress;
        pending --> evicted;
        in_progress --> finished;
        in_progress --> error;
        in_progress --> canceled;
    </div>
  </body>
</html>

## Lifecycle
<!DOCTYPE html>
<html lang="en">
  <head>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/mermaid/8.14.0/mermaid.min.js"></script>
  </head>
  <body>
    <div class="mermaid">
      flowchart TD
        finish((End))
        created("Created (Pending)")
        evicted(Evicted)
        error(Error)
        finished(Finished)
        canceled(Canceled)
        should_execute{Should Execute?}
        execution_succeeded{Success?}
        err_kind{Error Kind}
        execute[[Execute]]
        rollback[[Rollback]]
        canceled_by_user>Canceled By User]
        created --> should_execute;
        should_execute -- No --> evicted --> finish;
        should_execute -- Yes --> execute;
        execute --> execution_succeeded;
        execute --> canceled_by_user --> rollback;
        execution_succeeded -- Yes --> finished --> finish;
        execution_succeeded -- No --> rollback;
        rollback --> err_kind;
        err_kind -- Canceled --> canceled --> finish;
        err_kind -- Error --> error --> finish
    </div>
  </body>
</html>

## Lease
### What is the operation lease
Lease is a mechanism to track the operations' execution process and check if we can rely on the current/future operation state. 
### Why Operations have it
Sometimes, an operation might get stuck. It could happen, for example, if the worker crashes during the execution of an operation.
To keep track of operations, we assign each operation a Lease.
This Lease has a TTL (time to live). 

When the operation is being executed, this TTL is renewed each time the lease is about to expire while the operation is still in progress.
It'll be revoked once the operation is finished.

### Troubleshooting
If an operation is fetched and the TTL expired (the TTL is in the past), the operation probably got stuck, 
and we can't rely upon its current state, nor guarantee the required side effects of the execution or rollback have succeeded.

If an operation does not have a Lease, it either did not start at all (should be on the queue) or is already finished. 
An Active Operation without a Lease is at an invalid state.


>**⚠ Maestro do not have a self-healing routine yet for expired operations.**


### Operation Lease Lifecycle
<!DOCTYPE html>
<html lang="en">
  <head>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/mermaid/8.14.0/mermaid.min.js"></script>
  </head>
  <body>
    <div class="mermaid">
      flowchart TD
          finish_routine((End))
          start_routine((Start))
          finish((End))
          operation_finished{Op. Finished?}
          to_execute[Operation to Execute]
          grant_lease[[Grant Lease]]
          renew_lease[[Renew Lease]]
          revoke_lease[[Revoke Lease]]
          wait_for_ttl(Wait for TTL)
          execute(Execute Operation)
          to_execute --&gt; grant_lease;
          grant_lease --&gt; renew_lease_routine;
          grant_lease --&gt; execute;
          subgraph renew_lease_routine [ASYNC Renew Lease Routine]
              start_routine --&gt; wait_for_ttl;
              wait_for_ttl --&gt; operation_finished;
              operation_finished -- Yes --&gt; revoke_lease;
              operation_finished -- No --&gt; renew_lease --&gt; wait_for_ttl;
              revoke_lease --&gt; finish_routine;
          end
          renew_lease_routine --&gt; finish;
    </div>
  </body>
</html>


## Available Operations
For more details on how to use Maestro API, see [this section](https://topfreegames.github.io/maestro/OpenAPI/).

### **Create Scheduler**
- Accessed through the `POST /schedulers` endpoint.
  - Creates the scheduler structure for receiving rooms;
  - The scheduler structure is validated, but the game room is not;
  - If operation fails, rollback feature will delete anything created related to scheduler.

### **Create New Scheduler Version**
- Accessed through the `POST /schedulers/:schedulerName` endpoint.
  - Creates a validation room (deleted right after).
    If Maestro cannot receive pings (not forwarded) from validation game room, operation fails;
  - When this operation finishes successfully, it enqueues the "Switch Active Version".
  - If operation fails rollback routine deletes anything (except for the operation) created related to new version.

### **Switch Active Version**
- Accessed through `PUT /schedulers/:schedulerName` endpoint.
  - If it's a major change (anything under Scheduler.Spec changed), GRUs are replaced using scheduler **maxSurge** property;
  - If it's a minor change (Scheduler.Spec haven't changed), GRUs are **not** replaced;

### **Add Rooms**
- Accessed through `POST /schedulers/:schedulerName/add-rooms` endpoint.
  - If any room fail on creating, the operation fails and created rooms are deleted on rollback feature;

### **Remove Rooms**
- Accessed through `POST /schedulers/:schedulerName/remove-rooms` endpoint.
  - Remove rooms based on amount;
  - Rollback routine does nothing.