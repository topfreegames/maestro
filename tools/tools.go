//+build tools

package tools

import (
	- "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/google/wire/cmd/wire"
	_ "golang.org/x/tools/cmd/goimports"
)
