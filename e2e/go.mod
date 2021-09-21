module github.com/topfreegames/maestro/e2e

go 1.16

require (
	github.com/golang/protobuf v1.5.2
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.11.1
	github.com/topfreegames/maestro v0.0.0-00010101000000-000000000000
	github.com/go-redis/redis v6.15.9+incompatible
	k8s.io/apimachinery v0.22.1
	k8s.io/apiserver v0.22.1
	k8s.io/client-go v0.22.1
)

replace github.com/topfreegames/maestro => ../
