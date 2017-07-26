Maestro API
===========

All API responses include a `X-Maestro-Version` header with the current Maestro module version.

## Healthcheck

  ### Healthcheck

  `GET /healthcheck`

  Validates that the app is still up, including the database connection.

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "healthy": true
        }
      ```

  * Error Response

    It will return an error if it failed to connect to the database.

    * Code: `500`
    * Content:

    ```
      {
        "healthy": false
      }
    ```

## Room Management

  ### Ping

  `PUT /scheduler/:schedulerName/rooms/:roomName/ping`

  This route should be called every 10 seconds and serves as a keep alive sent by the GRU to Maestro.

  * Request

    ```
    {
      timestamp: [int]<seconds since epoch>,
      status:    [string]<room-status>
    }
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Address Polling

  `GET  /scheduler/:schedulerName/rooms/:roomName/address`

  This route should be polled by the GRU in order to obtain the room address (host ip and port).

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "host":  [string]<host ip>,
          "ports": [
            {
              "port": [int]<room port>,
              "name": [string]<port name>
            },...
          ]
        }
      ```

  * Error Response

    It will return an error if some error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Room ready

  `PUT /scheduler/:schedulerName/rooms/:roomName/status`

  This route should be called every time a room is ready to receive a match. You'll need to make sure it is only called after the room has its address.

  * Request

    ```
    {
      timestamp: [int]<seconds since epoch>,
      status:    [string]"ready"
    }
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Match started

  `PUT /scheduler/:schedulerName/rooms/:roomName/status`

  This route should be called every time a match is started. It'll indicate that this GRU is occupied and is not available for new matches.

  * Request

    ```
    {
      timestamp: [int]<seconds since epoch>,
      status:    [string]"occupied"
    }
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Match ended

  `PUT /scheduler/:schedulerName/rooms/:roomName/status`

  This route should be called every time a match is ended. It'll indicate that this GRU is no longer occupied and is available for new matches.

  * Request

    ```
    {
      timestamp: [int]<seconds since epoch>,
      status:    [string]"ready"
    }
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

## Scheduler Management:

  ### Create

  `POST /scheduler`

  This route creates a scheduler in Maestro using a provided YAML config.

  * Request

    ```
    {
      "name": "room-name",
      "game": "game-name",
      "image": "somens/someimage:v123",
      "affinity": "node-affinity",
      "ports": [
        {
          "containerPort": 5050,
          "protocol": "UDP",
          "name": "port1"
        },
        {
          "containerPort": 8888,
          "protocol": "TCP",
          "name": "port2"
        }
      ],
      "limits": {
        "memory": "128Mi",
        "cpu": "1"
      },
      "shutdownTimeout": 180,
      "autoscaling": {
        "min": 100,
        "up": {
          "delta": 10,
          "trigger": {
            "usage": 70,
            "time": 600
          },
          "cooldown": 300
        },
        "down": {
          "delta": 2,
          "trigger": {
            "usage": 50,
            "time": 900
          },
          "cooldown": 300
        }
      },
      "env": [
        {
          "name": "EXAMPLE_ENV_VAR",
          "value": "examplevalue"
        },
        {
          "name": "ANOTHER_ENV_VAR",
          "value": "anothervalue"
        }
      ],
      "cmd": [
        "./room-binary",
        "-serverType",
        "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
      ]
    }
    ```

  * Success Response
    * Code: `201`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```


  ### Delete

  `DELETE /scheduler/:schedulerName`

  This route deletes a scheduler in Maestro using the scheduler name.

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Update

  `PUT /scheduler/:schedulerName`

  This route updates a scheduler in Maestro using a provided YAML config.

  * Request

    ```
    {
      "name": "room-name",
      "game": "game-name",
      "image": "somens/someimage:v123",
      "ports": [
        {
          "containerPort": 5050,
          "protocol": "UDP",
          "name": "port1"
        },
        {
          "containerPort": 8888,
          "protocol": "TCP",
          "name": "port2"
        }
      ],
      "limits": {
        "memory": "128Mi",
        "cpu": "1"
      },
      "shutdownTimeout": 180,
      "autoscaling": {
        "min": 100,
        "up": {
          "delta": 10,
          "trigger": {
            "usage": 70,
            "time": 600
          },
          "cooldown": 300
        },
        "down": {
          "delta": 2,
          "trigger": {
            "usage": 50,
            "time": 900
          },
          "cooldown": 300
        }
      },
      "env": [
        {
          "name": "EXAMPLE_ENV_VAR",
          "value": "examplevalue"
        },
        {
          "name": "ANOTHER_ENV_VAR",
          "value": "anothervalue"
        }
      ],
      "cmd": [
        "./room-binary",
        "-serverType",
        "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
      ]
    }
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if scheduler to update is not found on DB

    * Code: `404`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if `:schedulerName` doesn't match name on config

    * Code: `400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Status

  `GET /scheduler/:schedulerName`

  Returns scheduler status and the room count for each status.

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "game":               [string]<game-name>,
          "state":              [string]<scheduler-state>,
          "stateLastChangedAt": [int]<timestamp when last change happened>,
          "lastScaleOpAt":      [int]<timestamp when last scale happened>,
          "roomsAtCreating":    [int]<number of rooms with creating status>,
          "roomsAtOccupied":    [int]<number of rooms with occupied status>,
          "roomsAtReady":       [int]<number of rooms with ready status>,
          "roomsAtTerminating": [int]<number of rooms with terminating status>
        }
      ```

  * Error Response

    It will return an error if scheduler is not found on DB

    * Code: `404`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if `:schedulerName` doesn't match name on config

    * Code: `400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Scheduler Config

  `GET /scheduler/:schedulerName?config`

  Returns scheduler config.

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "yaml": [string]<yaml-config>,
        }
      ```

  * Error Response

    It will return an error if scheduler is not found on DB

    * Code: `404`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Scale Up

  `POST /scheduler/:schedulerName?scaleup=amount`

  Manually scales up the number of rooms of schedulerName.

  * Request

    ```
    {}
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

  ### Scale Down

  `POST /scheduler/:schedulerName?scaledown=amount`

  Manually scales down the number of rooms of schedulerName.

  * Request

    ```
    {}
    ```

  * Success Response
    * Code: `200`
    * Content:

      ```
        {
          "success": true
        }
      ```

  * Error Response

    It will return an error if the request is invalid or the sent parameters are incorrect.

    * Code: `422`|`400`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        [string]<error-code>,
        "error":       [string]<error-message>,
        "description": [string]<error-description>,
        "success":     [bool]false
      }
    ```

