// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import (
	"bytes"
	"io"
	"os"
)

//MockStdout to read result from tests
func MockStdout() (io.WriteCloser, chan string) {
	r, w, _ := os.Pipe()
	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	return w, outC
}
