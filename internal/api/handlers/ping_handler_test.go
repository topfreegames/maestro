//+build integration

package handlers

import (
	"context"
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
		api.RegisterPingHandlerServer(context.Background(), mux, ProvidePingHandler())

		req, err := http.NewRequest("GET", "/ping", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		require.Equal(t, "{\"message\":\"pong\"}", bodyString)
	})

	t.Run("with invalid request method", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		mux := runtime.NewServeMux()
		api.RegisterPingHandlerServer(context.Background(), mux, ProvidePingHandler())

		req, err := http.NewRequest("POST", "/ping", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 405, rr.Code)
		bodyString := rr.Body.String()
		require.Equal(t, "Method Not Allowed\n", bodyString)
	})

	t.Run("with invalid request path", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		mux := runtime.NewServeMux()
		api.RegisterPingHandlerServer(context.Background(), mux, ProvidePingHandler())

		req, err := http.NewRequest("GET", "/pong", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
		bodyString := rr.Body.String()
		require.Equal(t, "Not Found\n", bodyString)
	})
}
