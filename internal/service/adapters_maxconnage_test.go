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

//go:build unit
// +build unit

package service

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	configmock "github.com/topfreegames/maestro/internal/config/mock"
)

func TestCreateRedisClient_MaxConnAge_AppliesWhenSet(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	config := configmock.NewMockConfig(mockCtrl)

	config.EXPECT().GetInt(redisPoolSizePath).Return(500)
	config.EXPECT().GetDuration(redisMaxConnAgePath).Return(5 * time.Minute)
	config.EXPECT().GetBool("api.tracing.jaeger.disabled").Return(true)

	client, err := createRedisClient(config, "redis://localhost:6379/0")
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, client.Options().MaxConnAge)
}

func TestCreateRedisClient_MaxConnAge_UnsetKeepsDefault(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	config := configmock.NewMockConfig(mockCtrl)

	config.EXPECT().GetInt(redisPoolSizePath).Return(500)
	config.EXPECT().GetDuration(redisMaxConnAgePath).Return(time.Duration(0))
	config.EXPECT().GetBool("api.tracing.jaeger.disabled").Return(true)

	client, err := createRedisClient(config, "redis://localhost:6379/0")
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), client.Options().MaxConnAge)
}
