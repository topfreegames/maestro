# Rolling Update

When the scheduler is updated, minor of major, the switch version operation will
simply change the new active version in the database. Thus, who is responsible for
actually performing runtime changes to enforce that all existing Game Rooms are from
the active schedulers is the _health_controller_ operation.

This has some benefits attached to it:

1. Consolidates all game room operations in the Add and Remove operations
2. If the update fails, rollback is more smooth. Previously, in the
_switch_version_ opeartion, if any creation/deletion of GRs failed, we would
sequentially delete all created game rooms. This can hog the worker in this
operation (no upscale during this time) and also the delete was not taking into
consideration ready target, thus it could heavily offend it
3. Each _health_controller_ loop is able to adjust how much game rooms it asks for
creation and process operations in between, avoiding the worker to be hogged in a
single operation

## Add Rooms Limit Impacts

Keep in mind that the _add_rooms_ operation has its own limit of rooms that can create
per operation, thus if more rooms than the limit is requested Maestro will cap to this
limit and on the next cycle a new set of rooms will be requested.

For example, if on the first cycle the autoscale asks for 1000 rooms and limit is set
to 150, then only 150 are created and in the next cycle 850 rooms (assuming the
autoscale compute is the same) will be requested - until the desired number of rooms is
reached.

## Old behavior

1. A new scheduler version is created or a call to switch the active version is made
2. _new_version_ operation starts by the worker
3. A new validation room is created to validate the scheduler config. If this room
becomes ready, the new version is marked as safe to be deployed
4. _switch_version_ operation starts by the worker, other operations will wait until
this one finishes (no autoscaling)
5. Maestro spawns a `maxSurge` amount of goroutines reading from a channel. Each
goroutine creates a game room with the new config and deletes a game room from
previous config
6. When all goroutines finish processing, meaning that there aren't any more GRs
from the old version to be deleted (channel closed), the operation changes the
active version in the database
7. Operation _switch_version_ finishes and other operations can run

If at any point there is an error in one of the goroutines or other part of the
process, worker starts to rollback. The rollback mechanism will delete all newly
created game rooms sequentially, even if this means that the system will have no
available game rooms by the end of it.

## New behavior

The new behavior is heavily inspired by how Kubernetes perform rolling updates in
deployments working gracefully alongside HPA (Horizontal Pod Autoscaler). Thus, two
parameters will guide this process:

* `maxUnavailable`: how many ready pods below the desired can exist at a time in the
deployment. In other words, how many game rooms below `readyTarget` policy we can
delete. For now, Maestro will use `maxUnavailable: 0`, which means we never go below
the `readyTarget` at any given cycle of the update. This can make the process a bit
slower, but it ensures higher availability.
* `maxSurge`: how many Game Rooms can be created at once (per cycle) and how many
Game Rooms can exist above the desired. The later concept is important to understand
that we should go above the desired, otherwise we can not spawn a new Game Room
since it would offend the autoscaler/HPA that would try to delete it

1. A new scheduler version is created or a call to switch the active version is made
2. _new_version_ operation starts by the worker
3. A new validation room is created to validate the scheduler config. If this room
becomes ready, the new version is marked as safe to be deployed
4. _switch_version_ operation starts by the worker, simply changing the active
version in the database
5. _health_controller_ operation runs and check if there is any existing game room
that is not from the active scheduler version
6. If it does not have, run normal autoscale. If it has, it is performing a rolling
update, proceed with the update
7. The update checks how many rooms it can spawn by computing the below
```
maxSurge * totalAvailableRooms (pending + unready + ready + occupied)
```
8. Enqueues a priority _add_room_ operation to create the surge amount
9. Check how many old rooms it can delete by computing
```
currentNumberOfReadyRooms - desiredAmountOfReadyRooms (autoscale)
```
10. If it can delete, enqueue a _delete_room_ operation. The above is valid for
`maxUnavailable: 0` so we never offend the `readyTarget`. Rooms are deleted by ID
since Maestro must delete only the rooms that are not from the active scheduler
version. Also, the occupied rooms will be the last one deleted from the list.
11. One _health_controller_ cycle finishes running the rolling update
12. _health_controller_ runs as many cycles as needed creating and deleting until
there are no more rooms from non-active scheduler versions to be deleted. When this
happens, rolling update finishes and _health_controller_ performs normal autoscale

# Scenarios

Below you will find how rolling update will perform in different scenarios when updating the schduler. Use as a reference to observe the behavior and tune parameters accordingly.

The scenarios assume that all rooms created in the surge will transition to ready
in the next loop, which usually runs each 30s to 1min (depends on the
configuration). In reality, depending on the number of rooms to surge, runtime might
take longer to provision a node or game code might take a while to initialize
and room actually becoming ready.

Also, the number of occupied rooms will remain the same, which means that when an
occupied Game Room from a previous version is deleted, a new one that was ready
transitions to occupied just for the sake of simplicity in the computation of
numbers of the scenario. In reality, the number of occupied rooms will vary
throughout the cycle and rolling update will adjust to that as well.

## Few Amount of Game Room

* readyTarget: 0.5
* maxSurge: 25%

### Downscale

| **loop** | **ready** | **occupied** | **available** | **desired** | **desiredReady** | **toSurge** | **toBeDeleted** |
|----------|-----------|--------------|---------------|-------------|------------------|-------------|-----------------|
| **1**    | 20        | 5            | 25 (0 new)    | 10          | 5                | 7           | 15              |
| **2**    | 12        | 5            | 17 (7 new)    | 10          | 5                | 4           | 7               |
| **3**    | 9         | 5            | 14 (11 new)   | 10          | 5                | 3           | 3 (4 actually)  |
| **4**    | 9         | 5            | 14 (14 new)   | 10          | 5                | -           | 4 by autoscale  |
| **5**    | 5         | 5            | 10 (10 new)   | 10          | 5                | -           | -               |

### Upscale

| **loop** | **ready** | **occupied** | **available** | **desired** | **desiredReady** | **toSurge** | **toBeDeleted** |
|----------|-----------|--------------|---------------|-------------|------------------|-------------|-----------------|
| **1**    | 5         | 20           | 25 (0 new)    | 40          | 20               | 7           | 0               |
| **2**    | 12        | 20           | 32 (7 new)    | 40          | 20               | 8           | 0               |
| **3**    | 20        | 20           | 40 (15 new)   | 40          | 20               | 10          | 0               |
| **4**    | 30        | 20           | 50 (25 new)   | 40          | 20               | 13          | 10              |
| **5**    | 33        | 20           | 53 (38 new)   | 40          | 20               | 14          | 13              |
| **6**    | 34        | 20           | 54 (52 new)   | 40          | 20               | 14          | 2 (actual 14)   |
| **7**    | 46        | 20           | 66 (66 new)   | 40          | 20               | -           | 26 by autoscale |
| **8**    | 20        | 20           | 40 (40 new)   | 40          | 20               | -           | -               |

## Big Amount of Game Rooms

### Upscale

* readyTarget: 0.4 -> 0.7
* maxSurge: 25%

| **loop** | **ready** | **occupied** | **available**   | **desired** | **desiredReady** | **toSurge** | **toBeDeleted**  |
|----------|-----------|--------------|-----------------|-------------|------------------|-------------|------------------|
| **1**    | 209       | 458          | 667 (0 new)     | 1526        | 1068             | 167         | 0                |
| **2**    | 376       | 458          | 834 (167 new)   | 1526        | 1068             | 209         | 0                |
| **3**    | 585       | 458          | 1043 (376 new)  | 1526        | 1068             | 261         | 0                |
| **4**    | 846       | 458          | 1304 (637 new)  | 1526        | 1068             | 326         | 0                |
| **5**    | 1172      | 458          | 1630 (963 new)  | 1526        | 1068             | 408         | 104              |
| **6**    | 1476      | 458          | 1934 (1371 new) | 1526        | 1068             | 484         | 408              |
| **7**    | 1552      | 458          | 2010 (1855 new) | 1526        | 1068             | 503         | 155              |
| **8**    | 2055      | 458          | 2513 (2513 new) | 1526        | 1068             | -           | 987 by autoscale |
| **9**    | 1068      | 458          | 1526 (1526 new) | 1526        | 1068             | -           | -                |

### Downscale

* readyTarget: 0.7 -> 0.4
* maxSurge: 25%

| **loop** | **ready** | **occupied** | **available**   | **desired** | **desiredReady** | **toSurge** | **toBeDeleted**   |
|----------|-----------|--------------|-----------------|-------------|------------------|-------------|-------------------|
| **1**    | 940       | 1040         | 1980 (0 new)    | 1733        | 693              | 495         | 247               |
| **2**    | 1188      | 1040         | 2228 (495 new)  | 1733        | 693              | 557         | 495               |
| **3**    | 1250      | 1040         | 2290 (1052 new) | 1733        | 693              | 573         | 557               |
| **4**    | 1266      | 1040         | 2306 (1625 new) | 1733        | 693              | 577         | 573               |
| **5**    | 1270      | 1040         | 2310 (2202 new) | 1733        | 693              | 578         | 108 (577)         |
| **6**    | 1740      | 1040         | 2780 (2780 new) | 1733        | 693              | -           | 1047 by autoscale |
| **7**    | 693       | 1040         | 1733 (1733 new) | 1733        | 693              | -           | -                 |
