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

package servermocks

import (
	"context"
	"fmt"

	"github.com/docker/compose/v2/pkg/api"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
)

type ServerMocks struct {
	GrpcForwarderAddress string
}

func ProvideServerMocks(ctx context.Context, compose tc.ComposeStack) (*ServerMocks, error) {
	services := compose.Services()
	services = append(services, "grpc-mock")

	err := compose.Up(ctx, tc.RunServices(services...), tc.Recreate(api.RecreateNever))
	if err != nil {
		return nil, fmt.Errorf("failed to start grpc-mock: %w", err)
	}

	return &ServerMocks{
		GrpcForwarderAddress: "grpc-mock:4770",
	}, nil
}
