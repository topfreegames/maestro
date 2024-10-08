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

package requestadapters

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func FromApiUpdateRoomRequestToEntity(request *api.UpdateRoomWithPingRequest) (*game_room.GameRoom, error) {
	status, err := game_room.FromStringToGameRoomPingStatus(request.GetStatus())
	if err != nil {
		return nil, err
	}

	return &game_room.GameRoom{
		ID:            request.GetRoomName(),
		SchedulerID:   request.GetSchedulerName(),
		PingStatus:    status,
		Metadata:      request.Metadata.AsMap(),
		LastPingAt:    time.Unix(request.GetTimestamp(), 0),
		OccupiedSlots: int(request.GetOccupiedSlots()),
	}, nil
}

func FromInstanceEntityToGameRoomAddressResponse(instance *game_room.Instance) *api.GetRoomAddressResponse {
	response := &api.GetRoomAddressResponse{
		Host:      instance.Address.Host,
		Ports:     []*api.Port{},
		Ipv6Label: "",
	}

	for _, port := range instance.Address.Ports {
		response.Ports = append(response.Ports, &api.Port{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: port.Protocol,
		})
	}
	return response
}
