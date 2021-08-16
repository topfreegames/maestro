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

//+build integration

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	configmock "github.com/topfreegames/maestro/internal/config/mock"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestPingHandler(t *testing.T) {

	t.Run("with valid request", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		mux := runtime.NewServeMux()
		err := api.RegisterPingServiceHandlerServer(context.Background(), mux, ProvidePingHandler())
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/ping", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t, "pong", body["message"])
	})

	t.Run("with invalid request method", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		mux := runtime.NewServeMux()
		err := api.RegisterPingServiceHandlerServer(context.Background(), mux, ProvidePingHandler())
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/ping", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 501, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t, "Method Not Allowed", body["message"])
	})

	t.Run("with invalid request path", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		mux := runtime.NewServeMux()
		err := api.RegisterPingServiceHandlerServer(context.Background(), mux, ProvidePingHandler())
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/pong", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t, "Not Found", body["message"])
	})
}
