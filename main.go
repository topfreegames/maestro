// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import (
	"C"
	"github.com/topfreegames/maestro/cmd"
)

func main() {
	cmd.Execute(cmd.RootCmd)
}
