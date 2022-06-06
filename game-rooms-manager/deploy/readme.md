Game rooms manager
================
Simple instructions to deploy game rooms manager into a kubernetes cluster to manage sandbox dedicated game servers
to test maestro behaviour in different scenarios.

Deploying to Kubernetes
-----------------------

```bash
> kubectl -n {scheduler-namespace} create -f deployment.yaml
> kubectl expose deployment game-rooms-manager --type=ClusterIP --port=8080 --target-port=8080
```

Use the following command in your game room so you can manage it remotely:
``` yaml
    "command": [
        "sh",
        "-c",
        "apk add curl jq && trap 'echo \"Deregistering room... \" && curl --location --request DELETE \"game-rooms-manager:8080/deregister/${MAESTRO_ROOM_ID}\" && echo \"Room deregistered!\"; exit' SIGTERM && curl --location --request POST \"game-rooms-manager:8080/register/${MAESTRO_ROOM_ID}\" && while true; do curl --request PUT 192.168.160.1:8070/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping --data-raw '{\"status\": '$(curl --location --request GET \"game-rooms-manager:8080/getStatus/${MAESTRO_ROOM_ID}\" | jq .\"status\")',\"timestamp\" : \"12312312313\"}' && sleep 5 & wait $!; done"
    ],
```