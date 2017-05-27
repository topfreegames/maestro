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
      timestamp: <seconds since epoch>,
      status: <room-status>
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
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
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
          "success": true,
          "host": <host ip>,
          "port": <int>
        }
      ```

  * Error Response

    It will return an error if some error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        "error-code",
        "error":       "error-message",
        "description": "error-description",
        "success":     false
      }
    ```

  ### Room ready

  `PUT /scheduler/:schedulerName/rooms/:roomName/status`

  This route should be called every time a room is ready to receive a match. You'll need to make sure it is only called after the room has its address.

  * Request

    ```
    {
      timestamp: <seconds since epoch>,
      status: "ready"
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
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

  ### Match started

  `PUT /scheduler/:schedulerName/rooms/:roomName/status`

  This route should be called every time a match is started. It'll indicate that this GRU is occupied and is not available for new matches.

  * Request

    ```
    {
      timestamp: <seconds since epoch>,
      status: "occupied"
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
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

  ### Match ended

  `PUT /scheduler/:schedulerName/rooms/:roomName/status`

  This route should be called every time a match is ended. It'll indicate that this GRU is no longer occupied and is available for new matches.

  * Request

    ```
    {
      timestamp: <seconds since epoch>,
      status: "ready"
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
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
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
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```

    It will return an error if some other error occurred.

    * Code: `500`
    * Content:

    ```
      {
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
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
        "code":        <error-code>,
    		"error":       <error-message>,
    		"description": <error-description>,
    		"success":     false
      }
    ```
