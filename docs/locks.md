Locks
===========

Maestro makes use of 3 different locks to mitigate concurrency between its operations:
`SchedulerScalingLock`, `SchedulerConfigLock` and `SchedulerDownScalingLock`

## SchedulerScalingLock

Is the main lock used by all operations (watcher and scheduler update).

## SchedulerConfigLock

This lock is used to block other scheduler operations when one is already happening.
This lock stays through all scheduler update operation flow until the rolling update is finished.

## SchedulerDownScalingLock

It is used to block downscaling from watcher's autoscaler and watcher's ensureCorrectRooms when a
rolling update is in place in a scheduler update. If a scheduler update does not need that pods
roll update, this lock is not used.

# UpdateSchedulerConfig Flow

We will call SchedulerScalingLock SL, SchedulerConfigLock CL and SchedulerDownScalingLock DL from now on.

```javascript
--- Acquire CL ---
  --- Acquire SL ---
    1 - load scheduler from database
    2 - check diff between new and old YAML
    3 - save new scheduler on database
  --- Release SL ---
  4 - if rolling update is not needed go to 9
  --- Acquire DL ---
    5 - divide current pods in chunks of size maxSurge (a percentage of current pods)
    *** Start goroutines ***
      6 - for each pod in chunk create a new pod and delete an old one
    *** End goroutines ***
    7 - if all chunks finished go to 9
    8 - if rolling times out or is canceled or error (when a pod fails to be created), do a rollback (start the flow again with old config)
  --- Release DL ---
  9 - update scheduler veersion status (deployed, canceled, error, ...) on database
--- Release CL ---
10 - end
```