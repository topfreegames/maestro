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

package suites

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestGetSchedulersInfo(t *testing.T) {
	t.Parallel()

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		t.Run("Should Succeed - Get schedulers without rooms", func(t *testing.T) {
			t.Parallel()

			firstScheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"get-scheduler-info-game-without-rooms",
				[]string{"sh", "-c", "while true; do sleep 1; done"},
			)
			require.NoError(t, err)

			getSchedulersRequest := &maestroApiV1.GetSchedulersInfoRequest{Game: firstScheduler.Game}
			getSchedulersResponse := &maestroApiV1.GetSchedulersInfoResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", firstScheduler.Game), getSchedulersRequest, getSchedulersResponse)

			require.NoError(t, err)
			require.NotEmpty(t, getSchedulersResponse.Schedulers, 1)
			require.Equal(t, firstScheduler.Name, getSchedulersResponse.Schedulers[0].Name)
			require.Equal(t, firstScheduler.Game, getSchedulersResponse.Schedulers[0].Game)
			require.Equal(t, firstScheduler.State, getSchedulersResponse.Schedulers[0].State)
			require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsReady)
			require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsPending)
			require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsOccupied)
			require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsTerminating)
		})

		t.Run("Should Succeed - Get schedulers returning schedulers and rooms info", func(t *testing.T) {
			t.Parallel()

			secondScheduler, err := createSchedulerWithRoomsAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				"get-schedulers-info-game-with-rooms",
				kubeClient,
			)

			require.NoError(t, err)

			require.Eventually(t, func() bool {
				getSchedulersRequest := &maestroApiV1.GetSchedulersInfoRequest{Game: secondScheduler.Game}
				getSchedulersResponse := &maestroApiV1.GetSchedulersInfoResponse{}

				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", secondScheduler.Game), getSchedulersRequest, getSchedulersResponse)
				require.NoError(t, err)

				require.NotEmpty(t, getSchedulersResponse.Schedulers)
				require.Equal(t, secondScheduler.Name, getSchedulersResponse.Schedulers[0].Name)
				require.Equal(t, secondScheduler.Game, getSchedulersResponse.Schedulers[0].Game)
				require.Equal(t, secondScheduler.State, getSchedulersResponse.Schedulers[0].State)
				require.Equal(t, secondScheduler.RoomsReplicas, getSchedulersResponse.Schedulers[0].RoomsReplicas)

				if getSchedulersResponse.Schedulers[0].RoomsReady != int32(2) {
					return false
				}

				require.EqualValues(t, int32(2), getSchedulersResponse.Schedulers[0].RoomsReady)
				require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsPending)
				require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsOccupied)
				require.EqualValues(t, 0, getSchedulersResponse.Schedulers[0].RoomsTerminating)

				return true
			}, 1*time.Minute, 10*time.Millisecond)
		})

		t.Run("Should Succeed - Get schedulers game without scheduler should return empty array", func(t *testing.T) {
			t.Parallel()

			inexistentGame := "NON-EXISTENT-GAME"

			getSchedulersRequest := &maestroApiV1.GetSchedulersInfoRequest{Game: inexistentGame}
			getSchedulersResponse := &maestroApiV1.GetSchedulersInfoResponse{}

			err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", inexistentGame), getSchedulersRequest, getSchedulersResponse)

			require.NoError(t, err)
			require.Len(t, getSchedulersResponse.Schedulers, 0)
		})
	})

}
