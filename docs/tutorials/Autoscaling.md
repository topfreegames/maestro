## Configuring Scheduler Autoscaling

### Prerequisites

- Have a game room container image that communicates with maestro through Maestro's rooms API

### Learning Outcomes

After finishing this tutorial you will understand how:

- to configure autoscaling policies for your scheduler

### What is
Autoscaling is an optional feature in which the user can choose and parametrize different autoscaling policies that maestro
will use to automatically scale the number of rooms in the scheduler.

Maestro has an internal process that periodically keeps checking if it needs to create or delete game rooms for the given scheduler,
if autoscaling is not configured or enabled, it will always try to maintain the current number of rooms equal to **roomsReplicas** scheduler property.
If autoscaling is configured **and** enabled, it will use the configured autoscaling policy to decide if it needs to **scale up** (create more rooms), 
**scale down** (delete rooms) or **do nothing**.

```mermaid
  flowchart TD
    finish((End))
    add_rooms_operation(Enqueue add rooms operation)
    remove_rooms_operation(Enqueue remove rooms)
    use_rooms_replicas(Use rooms replicas to calculate the desired number of rooms)
    autoscaling_enabled{Autoscaling configured and enabled?}
    decide_operation{Compare current number of rooms with the desired amount.}
    use_autoscaling[Use Autoscaling policy to calculate the desired number of rooms coerced in min-max range]
    autoscaling_enabled -- No --> use_rooms_replicas;
    autoscaling_enabled -- Yes --> use_autoscaling;
    use_autoscaling --> decide_operation;
    use_rooms_replicas --> decide_operation;
    decide_operation --  desired > actual --> add_rooms_operation --> finish;
    decide_operation --  desired == actual --> finish;
    decide_operation --  desired < actual --> remove_rooms_operation --> finish;
```

Currently, the sync interval is configured by environment variable `MAESTRO_WORKERS_HEALTHCONTROLLERINTERVAL`.

> By default, the scheduler does not have autoscaling configured.

### How to configure and enable autoscaling
To get autoscaling working in your scheduler, firstly you need to configure an autoscaling policy and enable it, this autoscaling
configuration resides in the root of the scheduler structure itself.

[comment]: <> (YAML version)
<details>
    <summary>YAML version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
name: String
game: String
...
autoscaling:
  enabled: true
  min: 1
  max: 10
  cooldown: 60
  policy:
    type: roomOccupancy
    parameters:
      ...
      // Will vary according to the policy type.
        </pre>
    </div>
</details>


[comment]: <> (JSON version)
<details>
    <summary>JSON version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
{
  "name": "test",
  "game": "multiplayer",
  ...
  "autoscaling": {
    "enabled": true,
    "min": 10,
    "max": 300,
    "cooldown": 60,
    "policy": {
      "type": "roomOccupancy",
      "parameters": {
        ...
        // Will vary according to the policy type.
      }
    }
  }
}
        </pre>
    </div>
</details>

- **enabled** [boolean]: A value that can be true or false, indicating if the autoscaling feature is enabled/disabled for the given scheduler. Default: false.
- **min** [integer]: Minimum number of rooms the scheduler should have, it must be greater than zero. For zero value, disable autoscaling and set "roomsReplicas" to 0.
- **max** [integer]: Maximum number of rooms the scheduler can have. It must be greater than min, or can be -1 (to have no limit).
- **cooldown** [integer]: The cooldown period (in seconds) between **downscaling** operations. This is useful to avoid scale down too fast, 
  which can cause a lot of rooms to be deleted in a short period of time.
- **policy** [struct] : This field holds information regarding the autoscaling policy that will be used if the autoscaling feature is enabled:
  - **type** [string]:  Define the policy type that will be used, must be one of the [policy types maestro provides](#policy-types).
  - **parameters** [struct]: This field will contain arbitrary fields that will vary according to the chosen [policy type](#policy-types).


-------

## Policy Types
Maestro has a set of predefined policy types that can be used to configure the autoscaling, each policy will implement
a specific strategy for calculating the desired number of rooms and will have its configurable parameters.

**Note:** Policy types are mutually exclusive. The parameters used will depend on the value of `autoscaling.policy.type`.

### Room Occupancy Policy
The basic concept of this policy is to scale the scheduler up or down based on the actual room occupancy rate, by defining a "buffer" percentage
of ready rooms that Maestro must keep. The desired number of rooms will be given by the following formula:

`desiredNumberOfRooms = ⌈(numberOfOccupiedRooms/ (1- readyTarget) )⌉`

So basically Maestro will constantly try to maintain a certain percentage of rooms in **ready** state, by looking at the
actual room occupancy rate (number of rooms in **occupied** state).

#### Room Occupancy Policy Parameters
- **readyTarget** [float]: The percentage (in decimal value) of rooms that Maestro should try to keep in **ready** state, must be a value greater than 0 and less than 1.
- **downThreshold** [float]: It adjusts how often maestro scale down Game Rooms, where 0.99 means that maestro will always scale down a Game Room when it is free (respecting the readyTarget), and a value close to 0 means that maestro will almost never scale down. Must be a value greater than 0 and less than 1.

#### Example

[comment]: <> (YAML version)
<details>
    <summary>YAML version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
name: String
game: String
...
autoscaling:
  enabled: true
  min: 1
  max: 10
  policy:
    type: roomOccupancy
    parameters:
      roomOccupancy:
        readyTarget: 0.5
        </pre>
    </div>
</details>

[comment]: <> (JSON version)
<details>
    <summary>JSON version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
{
  "autoscaling": {
    "enabled": true,
    "min": 10,
    "max": 300,
    "policy": {
      "type": "roomOccupancy",
      "parameters": {
        "roomOccupancy": {
          "readyTarget": 0.5
        }
      }
    }
  }
}
        </pre>
    </div>
</details>

Below are some simulated examples of how the room occupancy policy will behave:

> Note that the autoscaling decision will always be limited by the min-max values! .

| totalRooms | occupiedRooms | readyTarget | desiredNumberOfRooms | autoscalingDecision |
|:----------:|:-------------:|:-----------:|:--------------------:|:-------------------:|
|    100     |      80       |     0.5     |          160         |    Scale Up: +60    |
|    100     |      50       |     0.5     |          100         |    Do Nothing: 0    |
|    100     |      30       |     0.5     |          60          |   Scale Down: -40   |
|     50     |      40       |     0.3     |          58          |    Scale Up: +8     |
|     50     |      35       |     0.3     |          50          |    Do Nothing: 0    |
|     50     |      10       |     0.3     |          15          |   Scale Down: -35   |
|     10     |       5       |     0.9     |          50          |    Scale Up: +40    |
|     10     |       1       |     0.9     |          10          |    Do Nothing: 0    |
|     10     |       1       |     0.8     |           5          |   Scale Down: -5    |
|     5      |       5       |     0.1     |           6          |    Scale Up: +1     |
|     1      |       1       |     0.3     |           2          |    Scale Up: +1     |
|     2      |       2       |     0.9     |          20          |    Scale Up: +18    |

### Fixed Buffer Amount Policy
The Fixed Buffer Amount policy maintains a fixed number of rooms on top of the currently occupied rooms. This policy is useful when you want to ensure a consistent buffer of available rooms regardless of the occupancy rate.

The desired number of rooms will be given by the following formula:

```
desiredNumberOfRooms = numberOfOccupiedRooms + fixedBufferAmount
```

Respecting autoscaling boundaries (min and max):

```go
if desiredNumberOfRooms < autoscaling.Min {
  desiredNumberOfRooms = autoscaling.Min
}

if autoscaling.Max != -1 && desiredNumberOfRooms > autoscaling.Max {
  desiredNumberOfRooms = autoscaling.Max
}
```

Maestro will constantly try to maintain the specified fixed amount of rooms in addition to the occupied rooms, ensuring there are always enough rooms available for new players.

#### Fixed Buffer Policy Parameters

- **fixedBufferAmount** [integer]: The fixed number of rooms that Maestro should maintain on top of the occupied rooms. Must be a value greater than 0 and less than the maximum number of rooms (max) when max is greater than 0.

#### Example

[comment]: <> (YAML version)
<details>
    <summary>YAML version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
name: String
game: String
...
autoscaling:
  enabled: true
  min: 1
  max: 100
  policy:
    type: fixedBuffer
    parameters:
      fixedBuffer:
        amount: 50
        </pre>
    </div>
</details>

[comment]: <> (JSON version)
<details>
    <summary>JSON version</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
{
  "autoscaling": {
    "enabled": true,
    "min": 1,
    "max": 100,
    "policy": {
      "type": "fixedBuffer",
      "parameters": {
        "fixedBuffer": {
          "amount": 50
        }
      }
    }
  }
}
        </pre>
    </div>
</details>

[comment]: <> (Coexisting with RoomOccupancy)
<details>
    <summary>Coexisting with RoomOccupancy</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
{
  "autoscaling": {
    "enabled": true,
    "min": 1,
    "max": 100,
    "policy": {
      "type": "fixedBuffer",
      "parameters": {
        "fixedBuffer": {
          "amount": 50
        },
        "roomOccupancy": {
          "readyTarget": 0.2,
          "downThreshold": 0.9
        }
      }
    }
  }
}
        </pre>
    </div>
</details>

Below are some simulated examples of how the fixed buffer amount policy will behave:

> Note that the autoscaling decision will always be limited by the min-max values!

| totalRooms | occupiedRooms | fixedBufferAmount | desiredNumberOfRooms | autoscalingDecision |
|:----------:|:-------------:|:-----------------:|:--------------------:|:-------------------:|
|    100     |      80       |        50         |          130         |    Scale Up: +30    |
|    100     |      50       |        50         |          100         |    Do Nothing: 0    |
|    100     |      30       |        50         |           80         |   Scale Down: -20   |
|     50     |      40       |        20         |           60         |    Scale Up: +10    |
|     50     |      30       |        20         |           50         |    Do Nothing: 0    |
|     50     |      10       |        20         |           30         |   Scale Down: -20   |
|     10     |       5       |        10         |           15         |    Scale Up: +5     |
|     10     |       0       |        10         |           10         |    Do Nothing: 0    |
|     10     |       0       |         5         |            5         |   Scale Down: -5    |
|     5      |       5       |         5         |           10         |    Scale Up: +5     |
|     1      |       1       |         3         |            4         |    Scale Up: +3     |
|     2      |       2       |         8         |           10         |    Scale Up: +8     |

