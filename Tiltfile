if config.tilt_subcommand == "up":
    kind_clusters=local("kind get clusters")
    if str(kind_clusters).find("maestro") == -1:
        exit("No kind cluster named 'maestro' found.")
    local("kubectx kind-maestro")

# if config.tilt_subcommand == "down":
#     local("kind delete cluster -n maestro")

allow_k8s_contexts("kind-maestro")

docker_images=[
    "postgres:17.1-alpine",
    "redis:7.2.6-alpine",
]
for image in docker_images:
    local_resource(
        name="ensure local docker image" + image,
        cmd="docker pull " + image,
    )
    local_resource(
        name="load local docker image" + image,
        resource_deps=["ensure local docker image" + image],
        cmd="kind load docker-image " + image + " --name maestro",
    )

k8s_yaml("tilt/namespace.yaml")

k8s_yaml("./tilt/postgresql.yaml")
k8s_resource(
    workload="postgresql",
    port_forwards="5432"
)

k8s_yaml("./tilt/redis.yaml")
k8s_resource(
    workload="redis",
    port_forwards="6379"
)

local_resource(
    name="database migrations",
    resource_deps=["namespace", "postgresql"],
    cmd="go run main.go migrate",
    env={"MAESTRO_MIGRATION_PATH": "file://internal/service/migrations"}
)