// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import (
	"encoding/json"
	"io"
	"strings"
)

//JSON test type
type JSON map[string]interface{}

//JSONFor tests
func JSONFor(data JSON) io.Reader {
	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	return strings.NewReader(string(b))
}
