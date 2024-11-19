# Maestro Schedulers' best practices

## Scheduler rollouts

This document defines best practices for games on how to operate Maestro during scheduler’s transitions to avoid occurrences of no rooms available.
As a rule of thumb, the lower the maxSurge, the safer the rollout. Maestro creates and removes fewer % rooms (compared to the total) in each iteration (1 min interval by default). Thus, a maxSurge of 10% will take ~10-20 minutes to complete the rollout. If time is not a constraint, aim to use a lower maxSurge to ensure the smoothest process during major rollouts (any change within spec).

### 1. When Changing Schedulers:

#### Gradual Rollout:

**Scenario:** Players are gradually joining a new scheduler, possibly due to an Android or iOS canary release.

**Recommendation:** Increase the ready target to at least 40%. This ensures sufficient capacity during the transition, reducing the risk of room shortages as the new scheduler ramps up.

#### Forced Rollout:

**Scenario:** Players will be manually disconnected from the current scheduler, and all new matches will be routed to the new scheduler.

**Recommendation:** Set the minimum number of rooms to match or exceed the baseline of the previous scheduler. This provides a buffer to maintain stability during the transition.

### 2. Post-Rollout Adjustments:

#### Rollback Ready Target / Min Room Count:

Once the rollout has finished, you can rollback the ready target and/or minimum number of rooms to their standard levels.

**Zooba:**
- US east: 115 min rooms;
- AP: 60 min rooms;

**WM:**
- EU: 250 min rooms;
- AP: 50 min rooms;

**Sniper:**
- US east: 8 min rooms;

**Sky Warriors:**
- US east: 37 min rooms;
- EU: 35 min rooms;
- AP: 28 min rooms;
