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


## Recommended defaults for games

This section outlines a few recommended defaults games that uses Maestro.These are only recommendations, and they can definitely change if your game's requirements change. Use them as a guideline to define what works best for you.

###  Minimum number of rooms

They were defined by getting the minimum number of rooms that each region had during a 1 week avaliation window.

| Game         | Region | Min |
|--------------|--------|-----|
| Zooba        | US     | 115 |
| Zooba        | AP     | 60  |
| War Machines | EU     | 250 |
| War Machines | AP     | 50  |
| Sniper3d     | US     | 8   |
| Sky Warriors | US     | 37  |
| Sky Warriors | EU     | 35  |
| Sky Warriors | AP     | 28  |
