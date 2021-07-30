//+build tools

package tools

import (
	_ "github.com/bufbuild/buf/cmd/buf"
	- "github.com/golang-migrate/migrate/v4/cmd/migrate"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/wire/cmd/wire"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
    _ "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
)
 