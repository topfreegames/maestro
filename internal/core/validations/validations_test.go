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

package validations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsMaxSurgeValid(t *testing.T) {
	t.Run("with success when max surge is a number greater than zero", func(t *testing.T) {
		maxSurgeValid := IsMaxSurgeValid("5")
		assert.True(t, maxSurgeValid)
	})

	t.Run("with success when maxSurge is a number greater than zero with suffix '%'", func(t *testing.T) {
		maxSurgeValid := IsMaxSurgeValid("5%")
		assert.True(t, maxSurgeValid)
	})

	t.Run("fails when maxSurge is empty", func(t *testing.T) {
		maxSurgeValid := IsMaxSurgeValid("")
		assert.False(t, maxSurgeValid)
	})

	t.Run("fails when maxSurge is less or equal zero", func(t *testing.T) {
		maxSurgeValid := IsMaxSurgeValid("0")
		assert.False(t, maxSurgeValid)
	})
}

func TestIsImagePullPolicySupported(t *testing.T) {
	t.Run("with success when policy is supported by maestro", func(t *testing.T) {
		supported := IsImagePullPolicySupported("Always")
		assert.True(t, supported)
	})

	t.Run("fails when policy is not supported by maestro", func(t *testing.T) {
		wrongPolicy := "unsupported"
		supported := IsImagePullPolicySupported(wrongPolicy)
		assert.False(t, supported)
	})
}

func TestIsProtocolSupported(t *testing.T) {
	t.Run("with success when port protocol is supported by maestro", func(t *testing.T) {
		supported := IsProtocolSupported("tcp")
		assert.True(t, supported)
	})

	t.Run("fails when port protocol is not supported by maestro", func(t *testing.T) {
		wrongProtocol := "unsupported"
		supported := IsProtocolSupported(wrongProtocol)
		assert.False(t, supported)
	})
}

func TestIsVersionValid(t *testing.T) {
	t.Run("with success when semantic version is valid", func(t *testing.T) {
		valid := IsVersionValid("1.0.0-rc")
		assert.True(t, valid)
	})

	t.Run("fails when policy is not supported by maestro", func(t *testing.T) {
		invalid := IsVersionValid("0x0x0")
		assert.False(t, invalid)
	})
}

func TestIsKubeResourceNameValid(t *testing.T) {
	t.Run("should succeed - name is valid", func(t *testing.T) {
		nameMinLength := "a"
		usualName := "pod-sdf123-g1"
		nameMaxLength := "pod-bgvaoifdbgalsidubalisdfgalisdfbgaidsfgyubalosidbyasolfugyba"

		assert.True(t, IsKubeResourceNameValid(nameMinLength))
		assert.True(t, IsKubeResourceNameValid(usualName))
		assert.True(t, IsKubeResourceNameValid(nameMaxLength))
	})

	invalidCharactersBorders := "-$*_./#%()&^'\"\\Ωœ∑ø˚¬≤µ˜∫≈çå"
	invalidCharactersMiddle := "$*_./#%()&^'\"\\Ωœ∑ø˚¬≤µ˜∫≈çå"
	t.Run("should fail - name starts with invalid characters", func(t *testing.T) {
		for _, char := range invalidCharactersBorders {
			name := fmt.Sprintf("%cpod", char)
			assert.False(t, IsKubeResourceNameValid(name))
		}
	})

	t.Run("should fail - name have invalid characters", func(t *testing.T) {
		for _, char := range invalidCharactersMiddle {
			name := fmt.Sprintf("pod%cpod", char)
			assert.False(t, IsKubeResourceNameValid(name))
		}
	})

	t.Run("should fail - name ends with invalid characters", func(t *testing.T) {
		for _, char := range invalidCharactersBorders {
			name := fmt.Sprintf("pod%c", char)
			assert.False(t, IsKubeResourceNameValid(name))
		}
	})

	t.Run("should fail - name is too long", func(t *testing.T) {
		nameMaxLengthPlusOne := "pod-bgvaoifdbgalsidubalisdfgalisdfbgaidsfgyubalosidbyasolfugyba1"
		assert.False(t, IsKubeResourceNameValid(nameMaxLengthPlusOne))
	})
}

func TestIsForwarderTypeSupported(t *testing.T) {
	t.Run("with success when type is grpc", func(t *testing.T) {
		supported := IsForwarderTypeSupported("gRPC")
		assert.True(t, supported)
	})

	t.Run("fails when type is not supported by maestro", func(t *testing.T) {
		wrongType := "unsupported"
		supported := IsForwarderTypeSupported(wrongType)
		assert.False(t, supported)
	})
}
