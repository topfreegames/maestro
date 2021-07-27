//+build tools

package tools

import (
	_ "github.com/golang/mock/mockgen"
	_ "github.com/google/wire/cmd/wire"
	_ "golang.org/x/tools/cmd/goimports"
)
