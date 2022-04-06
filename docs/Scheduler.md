Table of Contents
---
- [What Is](#what-is)
  - [Versions](#versions)
  - [How to Operate](#how-to-operate)
  - [Example](#example)

## What is
Objectively, a **Scheduler** is 1:1 to **Kubernetes namespace** (as referenced [here](Kubernetes.md)), and contains all the information for creating 
game rooms and forwarding rooms information to other services.
The scheduler, then, is like a template for kubernetes.

The `create scheduler` operation stores the scheduler with all it's info,
and creates a namespace on Kubernetes with the name of the scheduler.

### Structure
The scheduler is represented as:
```yaml
name: String
game: String
state: String
portRange: PortRange
maxSurge: String
spec: Spec
forwarders: Forwarders
```

- **Name**: Scheduler name. This name is unique and will be the same name used for the kubernetes namespace.
  It's offered by the user in the creation and cannot be changed in the future. It's used as an ID for the scheduler.
- **game**: Name of the game which will use the scheduler. The game is important since it's common to use multiple schedulers
for a specific game. So you probably will want to fetch all the schedulers from a game.
## Versions
A Scheduler have versions, and each time we want to change scheduler properties, we end-up creating a new version to it.
> ⚠ Versions are directly calculated by Maestro, not sent by the client.
> 
> The client can only switch the active version based on the versions created by Maestro. To switch to an specific version, see [this](Operations.md#available-operations).

This version can either be a Minor or a Major change.

- Major version: **Replace** the game rooms. 
  - Basically, any change under **spec**, that are related to the game room directly.
- Minor version: **Don't replace** game rooms. 
  - Info such as MaxSurge or forwarders, that do not impact the game rooms.

## How to Operate
To directly interact with a Scheduler, the client enqueues [operations](Operations.md).

These operations are responsible for creating a scheduler or newer versions, switching an active version, switch version, 
adding/removing rooms, etc.

Because of that, everything that happens for a Scheduler can be tracked based on history of the operations executed for 
that scheduler and the order they were executed.


## Example
A complete Scheduler looks like this:

[comment]: <> (YAML scheduler)
<details>
    <summary>YAML</summary>
    <div class="highlight highlight-source-yaml position-relative overflow-auto">
        <pre>
name: scheduler-test
game: game-test
state: creating
portRange:
  start: 40000
  end: 60000
maxSurge: 30%
spec:
  terminationGracePeriod: '100'
  containers:
    - name: alpine
      image: alpine
      imagePullPolicy: IfNotPresent
      command:
        - /bin/sh
        - '-c'
        - >-
          apk add curl && while true; do curl --request POST
          {{maestro-rooms-api}}/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping
          --data-raw '{"status": "ready","timestamp": "12312312313"}' && sleep
          1; done
      environment:
        - name: env-var-name
          value: env-var-value
        - name: env-var-field-ref
          valueFrom:
            fieldRef:
              fieldPath: path
        - name: secret-var-name
          valueFrom:
            secretKeyRef:
              name: secret-name
              key: secret-key
      requests:
        memory: 20Mi
        cpu: 100m
      limits:
        memory: 200Mi
        cpu: 200m
      ports:
        - name: port-name
          protocol: tcp
          port: 12345
  toleration: maestro
  affinity: maestro-dedicated
forwarders:
  - name: test
    enable: true
    type: gRPC
    address: {{host}}
    options:
      timeout: '1000'
      metadata: {}
        </pre>
    </div>
</details>

[comment]: <> (JSON scheduler)
<details>
    <summary>JSON</summary>
    <pre>
{
    "name": "scheduler-test",
    "game": "game-test",
    "portRange": {
        "start": 40000,
        "end": 60000
    },
    "maxSurge": "30%",
    "spec": {
        "terminationGracePeriod": '100',
        "containers": [
            {
                "name": "alpine",
                "image": "alpine",
                "imagePullPolicy": "IfNotPresent",
                "command": [
                    "/bin/sh",
                    "-c",
                    "apk add curl && while true; do curl --request POST {{maestro-rooms-api}}/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping --data-raw '{"status": "ready","timestamp": "12312312313"}' && sleep 1; done"
                ],
                "environment": [
                    {
                        "name": "env-var-name",
                        "value": env-var-value
                    },
                    {
                        "name": "env-var-field-ref",
                        "valueFrom": {
                            "fieldRef": {
                                "fieldPath": "path"
                            }
                        }
                    },
                    {
                        "name": "secret-var-name",
                        "valueFrom": {
                            "secretKeyRef": {
                                "name": "secret-name",
                                "key": "secret-key"
                            }
                        }
                    }
                ],
                "requests": {
                    "memory": "20Mi",
                    "cpu": "100m"
                },
                "limits": {
                    "memory": "200Mi",
                    "cpu": "200m"
                },
                "ports": [
                    {
                        "name": "port-name",
                        "protocol": "tcp",
                        "port": 12345,
                    }
                ]
            }
        ],
        "toleration": "maestro",
        "affinity": "maestro-dedicated"
    },
    "forwarders": [
        {
            "name": "test",
            "enable": true,
            "type": "gRPC",
            "address": "{{host}}",
            "options": {
                "timeout": '1000',
                "metadata": {}
            }
        }
    ]
}
    </pre>
</details>
