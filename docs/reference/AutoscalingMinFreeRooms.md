# Autoscaling with minFreeRooms (Proposal)

## Background

Maestro operates game servers. Each game server runs in a room. A room is one Kubernetes pod.

A scheduler is the configuration for one fleet of rooms. This fleet belongs to one game in one region.

Each room has a status. For autoscaling, the statuses form four groups:

* **Free** (`ready`): The room has 0 running matches. The matchmaker can send players to this room.
* **Hosting matches** (`active`, `occupied`, or `allocated`): The room has 1 or more running matches (`active` when the room has space for more matches, `occupied` when the room is full). Or Maestro reserved the room for a match, and the room did not report the match yet (`allocated`).
* **Starting** (`pending`, `unready`): The pod exists, but the room is not ready to receive players yet. A room in this group is not free and does not host matches. Maestro counts it in the current total of rooms. A room must become `ready` within 2 minutes (`roomInitializationTimeoutMillis: 120000`). If it does not, Maestro removes it.
* **Not counted** (`terminating`, `error`, `terminated`): Maestro does not count these rooms in the current total.

Autoscaling is the Maestro function that automatically adds and removes rooms. The health controller runs this function every minute (`healthControllerInterval: 1m`). Every minute, the health controller counts the rooms. It calculates the target total of rooms with a policy. Then, it creates or removes rooms to reach this total.

A scheduler has one policy and two limits:

* **Min:** The minimum total of rooms. Maestro does not decrease the total below this limit.
* **Max:** The maximum total of rooms. Maestro does not increase the total above this limit.

## Current Policies

A scheduler uses only one of these two policies:

1. **Room Occupancy:** You specify a percentage (`readyTarget`). Maestro keeps this percentage of the total rooms free. For schedulers with 1 match for each room, the formulas for the target total and the free rooms are:

```text
total = occupied / (1 - readyTarget)
free  = total - occupied
```

If `readyTarget` is 0.2, the free rooms are 20% of the total, and the occupied rooms are 80% of the total. Therefore, the number of free rooms is always one quarter of the occupied rooms. For schedulers with more than 1 match for each room, Maestro applies the formula to the running matches and divides the result by `maxMatches`.

2. **Fixed Buffer:** You specify a number (`amount`). Maestro keeps this number of free rooms in addition to the rooms that host matches. The formulas are:

```text
total = hosting + amount
free  = amount
```

## Problem

The Room Occupancy buffer is a percentage. Therefore, the number of free rooms is small when few players are online. The number is large when many players are online. A sudden increase in new players has approximately the same size at all times of the day.

For example, a scheduler uses Room Occupancy with a `readyTarget` of 0.2 and a `min` of 300. Its daily load has this shape (hourly averages):

| Time of day | Occupied | Total | Free | Decided by |
| --- | --- | --- | --- | --- |
| **Quietest hour** | 159 | 300 | 141 | Min: the percentage wants 199, and Min lifts the total to 300 |
| **Morning ramp** | 228 to 282 | 304 to 368 | 75 to 86 | Percentage: the total is just above Min, and 20% of a small total is small |
| **Peak hour** | 940 | 1196 | 256 | Percentage |

The busiest minute has 1020 occupied rooms.

The buffer is smallest in the morning ramp, not in the quietest hour. In the quietest hour, Min holds the total at 300 and gives 141 free rooms. When the occupied rooms pass 240 (300 × 0.8), the percentage decides, and the free rooms fall to one quarter of the occupied rooms: 60 free rooms at 240 occupied.

During the peak hour, 256 free rooms receive 100 new matches without a problem. During the morning ramp, 100 new matches use all 75 free rooms in less than 1 minute. The health controller identifies this shortage at its next run. The new rooms need up to 2 more minutes to start. During this delay, new players wait for a room.

A higher Min does not solve this. Min lifts the total, and the free rooms are the total minus the occupied rooms. Min gives many free rooms in the quietest hour and progressively fewer as the occupied rooms increase, until the percentage takes over.

The Fixed Buffer policy with an `amount` of 300 gives 300 free rooms at all times. But the buffer does not increase with the load. At 2000 occupied rooms, Room Occupancy gives 500 free rooms and Fixed Buffer gives 300.

This scheduler needs both: a 20% buffer when many players are online, and a minimum of 300 free rooms at all other times. Today, no policy gives both.

## Proposal

Add the optional field `autoscaling.policy.parameters.roomOccupancy.minFreeRooms`. This field is an integer. The default value is 0.

The target total becomes the larger of these two values:

```text
percentageTotal = occupied / (1 - readyTarget)      # The existing Room Occupancy formula
floorTotal      = hosting + minFreeRooms
targetTotal     = max(percentageTotal, floorTotal)  # Maestro then applies the Min and Max limits
```

The free rooms become the larger of the percentage value and the `minFreeRooms` value. The floor decides when few players are online. The percentage decides when many players are online. The two values are equal when the occupied rooms are `minFreeRooms × (1 - readyTarget) / readyTarget`. For a `readyTarget` of 0.2 and a `minFreeRooms` of 300, this is 1200 occupied rooms.

**Downscale Operation:**
When the target total is less than the current total, Maestro removes rooms. The current condition that permits a downscale (`desiredFreeSlots / freeSlots <= downThreshold`) remains the same. `desiredFreeSlots` is the number of free slots that the fleet has when the total is equal to the target total. When the percentage decides the target total, this number is `targetTotal × maxMatches × readyTarget`. This is the current formula. When the floor decides the target total, this number is `targetTotal × maxMatches − running matches`. With 1 match for each room, this number is equal to `minFreeRooms`.

The floor formula is necessary for the downscale condition. When the floor decides the target total, the free rooms do not depend on `readyTarget`. For example, a scheduler has 240 occupied rooms, a `readyTarget` of 0.2, and a `minFreeRooms` of 300. The target total is 540 and the free rooms are 300. The percentage formula gives 108 free slots. With 108, the ratio `desiredFreeSlots / freeSlots` is 0.36 when the current total is equal to the target total. Every `downThreshold` greater than 0.36 permits a downscale when the current total is greater than the target total by 1 room. `downThreshold` has no effect. With 300, 5 rooms above the target total give a ratio of 0.98. A `downThreshold` of 0.7 does not permit a downscale. Maestro keeps the 5 rooms. This is the same result as when the percentage decides the target total.

Maestro performs a new check before this condition: if the number of free rooms is equal to or less than `minFreeRooms`, Maestro does not remove rooms. This check does not guarantee the floor, for two reasons:

* The health controller counts the rooms one time, makes the decision, and creates a remove operation. The remove operation runs after the health controller, and it does not count the rooms again. The occupied rooms can increase between the decision and the removal. Then, the free rooms are less than `minFreeRooms` until the next run of the health controller.
* The health controller includes `unready` rooms in the current total, but the remove operation does not select `unready` rooms. The remove operation selects rooms in this order: `error`, rooms with an expired ping, `pending`, `ready`, `occupied`. Thus, each `unready` room causes the removal of 1 more `ready` room. While rooms start, the free rooms can decrease below `minFreeRooms` by the number of `unready` rooms.

The percentage buffer (`readyTarget`) and the Fixed Buffer `amount` have the same limitations today. Each remove operation removes a maximum of 50 rooms.

**Default Behavior:**
The default value causes no changes. If `minFreeRooms` is 0, the policy does not apply the floor. The results remain the same as the current behavior. Schedulers that do not use this field have no changes. The current test suite passes without changes.

**Rooms with More Than One Match:**
Some games run more than one match in a single room (`maxMatches > 1`). For these schedulers, Maestro finds the room status from the running match count in the room ping:

* 0 matches is `ready`.
* 1 to `maxMatches - 1` matches is `active`.
* `maxMatches` matches is `occupied`.

Therefore, the floor counts only the rooms that have 0 matches. The matchmaker script of the game does not change this logic. If a script fills the existing rooms before it uses an empty room, *N* empty rooms give a larger reserve than *N* free slots. This is the safe direction. Maestro can get a separate field for a slot-based reserve in the future, if a game needs it.

## Examples

### Example 1: One Match for Each Room

`readyTarget` of 0.2, `min` of 300, `minFreeRooms` of 300, and 1 match for each room. The occupied values 159, 240, 344, and 1020 come from the daily load in **Problem**. The values 1200 and 1600 show the behavior if the game grows.

| Occupied | Current (Total / Free) | Proposed (Total / Free) | Deciding Factor |
| --- | --- | --- | --- |
| **0** | 300 / 300 | 300 / 300 | Min limit |
| **159** (quietest hour) | 300 / 141 | 459 / 300 | Floor |
| **240** (smallest buffer today) | 300 / 60 | 540 / 300 | Floor |
| **344** | 430 / 86 | 644 / 300 | Floor |
| **1020** (peak minute) | 1275 / 255 | 1320 / 300 | Floor |
| **1200** | 1500 / 300 | 1500 / 300 | Equal |
| **1600** | 2000 / 400 | 2000 / 400 | Percentage |

**Explanation of the results:**

* **At 159 occupied:** The percentage formula requires 199 rooms. The Min limit of 300 increases this total to 300. This leaves 141 free rooms. The proposed floor requires 159 + 300 = 459 rooms. This leaves 300 free rooms.
* **At 240 occupied:** The percentage formula requires exactly 300 rooms, the same as the Min limit. This leaves 60 free rooms. This is the smallest buffer with the current configuration. The proposed floor requires 540 rooms. This leaves 300 free rooms.
* **At 344 occupied:** The percentage formula requires 430 rooms. The floor formula requires 644 rooms. The floor is larger. Maestro plans 644 rooms, which leaves 300 free rooms.
* **At 1020 occupied:** The percentage formula requires 1275 rooms. The floor formula requires 1320 rooms. The floor is still larger. With this load, the floor decides at all times of the day.
* **At 1200 occupied:** Both formulas require 1500 rooms. Above this point, the percentage decides.
* **At 1600 occupied:** The percentage formula requires 2000 rooms. The floor formula requires 1900 rooms. The percentage is larger. Maestro plans 2000 rooms, which leaves 400 free rooms, the same as the current behavior.

The Max limit applies after both formulas. Near the Max limit, the free rooms decrease with both formulas. The floor cannot exceed the Max limit.

### Example 2: Multiple Matches per Room

Assume 13 matches for each room, `minFreeRooms` is 3, and 26 matches are running.

* If the 26 matches run in 2 rooms, the 2 rooms are `occupied`. The policy requires 5 rooms total (2 hosting + 3 floor). The percentage formula alone requires only 3 rooms.
* If the 26 matches run in 26 different rooms, the 26 rooms are `active`. The policy requires 29 rooms (26 hosting + 3 floor).
* If 39 free slots exist in rooms that host matches, and no room has 0 matches, the downscale check stops the removal of rooms. Maestro does not count these slots as free rooms.

> **Note:** A starting room is not a free room, and it is not a room that hosts matches. After an increase in rooms, the downscale check stops the removal of rooms while the `ready` rooms are equal to or less than `minFreeRooms`. If the `ready` rooms are more than `minFreeRooms`, Maestro can remove rooms while the new pods start (see **Downscale Operation**).
