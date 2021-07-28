//+build tools

package tools

import (
	_ "honnef.co/go/tools/cmd/staticcheck"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/google/wire/cmd/wire"
	_ "golang.org/x/tools/cmd/goimports"
)
