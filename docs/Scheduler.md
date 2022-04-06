Table of Contents
---
- [What Is](#what-is)
  - [Versions](#versions)
  - [Operations](#operations)
  - [Example](#example)

## What is
Objectively, a **Scheduler** is 1:1 to **Kubernetes namespace**, and contains all the information for creating game rooms
and forwarding rooms information to other services.

The `create scheduler` operation stores the scheduler with all it's info,
and creates a namespace on Kubernetes with the name of the scheduler.

### Versions
A Scheduler have versions, and each time we want to change scheduler properties, we end-up creating a new version to it.
> Versions are directly calculated by Maestro, not sent by the client.
> 
> The client can only switch the active version based on the versions created by Maestro.

This version can either be a Minor or a Major change.
- Major version: Replace the game rooms. 
  - Basically, any change under **spec**, that are related to the game room directly.
- Minor version: **Don't** replace game rooms. 
  - Info such as MaxSurge or forwarders, that do not impact the game rooms.

### Operations

### Example
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
