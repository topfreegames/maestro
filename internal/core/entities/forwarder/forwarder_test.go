// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package forwarder_test

import (
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/validations"
)

func TestNewForwarder(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	fwdOptions := &forwarder.FwdOptions{
		Timeout:  time.Second * 5,
		Metadata: nil,
	}

	t.Run("with success when create valid forwarder", func(t *testing.T) {
		forwarderName := "name"
		forwarderType := forwarder.TypeGrpc
		forwarderAddress := "127.0.0.1:6363"
		fwd, err := forwarder.NewForwarder(forwarderName, true, forwarderType, forwarderAddress, fwdOptions)

		require.NoError(t, err)
		require.NotNil(t, fwd)
	})

	t.Run("with error creating invalid forwarder", func(t *testing.T) {
		forwarderName := "name"
		forwarderType := forwarder.TypeGrpc
		forwarderAddress := ""
		fwd, err := forwarder.NewForwarder(forwarderName, true, forwarderType, forwarderAddress, fwdOptions)

		require.Error(t, err)
		require.Nil(t, fwd)
	})
}
