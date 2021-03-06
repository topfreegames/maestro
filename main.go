// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import (
	"github.com/topfreegames/maestro/cmd"
	_ "net/http/pprof"
)

func main() {
	cmd.Execute(cmd.RootCmd)
}
