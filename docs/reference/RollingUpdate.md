# Rolling Update

When the scheduler is updated, only minor, the switch version operation will
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
that is not from the active scheduler version. More details on how we check for it below.
   1. Get all rooms from that scheduler
   2. For each room check if the version is not equal to the current active scheduler version
   3. Check if we have compared with this version before. If not, get the scheduler entity for that old version
   4. Compare the old version with the new one, if it's a major one add it to the comparison map
   5. At the end of the rooms loop, if we do not find any major change return false when checking for a rolling update. If we found a major change, append the occupied and then ready rooms to the array
6. If it does not have, run normal autoscale. If it has, it is performing a rolling
update, proceed with the update
7. The update checks how many rooms it can spawn by computing the below:
   1. Calculate the current `deviation` (`totalRoomsAmount - desiredAmountOfRooms`);
   2. Calculate the `surge` amount over the desired (`surge = (maxSurge / 100) * desiredAmountOfRooms`);
   3. Discount the deviation from the surge (`surge = surge - deviation`);
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
| **1**    | 20        | 5            | 25 (0 new)    | 10          | 5                | 0           | 15              |
| **2**    | 5         | 5            | 10 (0 new)    | 10          | 5                | 2           | 0               |
| **3**    | 7         | 5            | 12 (2 new)    | 10          | 5                | 0           | 2               |
| **4**    | 5         | 5            | 10 (2 new)    | 10          | 5                | 2           | 0               |
| **5**    | 7         | 5            | 12 (4 new)    | 10          | 5                | 0           | 2               |
| **6**    | 5         | 5            | 10 (4 new)    | 10          | 5                | 2           | 0               |
| **7**    | 7         | 5            | 12 (6 new)    | 10          | 5                | 0           | 2               |
| **8**    | 5         | 5            | 10 (6 new)    | 10          | 5                | 2           | 0               |
| **9**    | 7         | 5            | 12 (8 new)    | 10          | 5                | 0           | 2               |
| **10**   | 5         | 5            | 10 (8 new)    | 10          | 5                | 2           | 0               |
| **11**   | 7         | 5            | 12 (10 new)   | 10          | 5                | -           | 2               |
| **12**   | 5         | 5            | 10 (10 new)   | 10          | 5                | -           | -               |

### Upscale

| **loop** | **ready** | **occupied** | **available** | **desired** | **desiredReady** | **toSurge** | **toBeDeleted** |
|----------|-----------|--------------|---------------|-------------|------------------|-------------|-----------------|
| **1**    | 5         | 20           | 25 (0 new)    | 40          | 20               | 10          | 0               |
| **2**    | 15        | 20           | 35 (10 new)   | 40          | 20               | 10          | 0               |
| **3**    | 25        | 20           | 45 (20 new)   | 40          | 20               | 5           | 5               |
| **4**    | 25        | 20           | 45 (25 new)   | 40          | 20               | 5           | 5               |
| **5**    | 25        | 20           | 45 (30 new)   | 40          | 20               | 5           | 5               |
| **6**    | 35        | 20           | 45 (35 new)   | 40          | 20               | 5           | 5               |
| **7**    | 25        | 20           | 45 (40 new)   | 40          | 20               | 0           | 5               |
| **8**    | 20        | 20           | 40 (40 new)   | 40          | 20               | 0           | 0               |
