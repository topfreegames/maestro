// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import (
	"fmt"
	pb "github.com/topfreegames/maestro/eventforwarder/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

type RoomStatusClient struct {
	roomStatus *pb.RoomStatus
}

func NewRoomStatus(game, roomId, roomType string) *RoomStatusClient {
	return &RoomStatusClient{
		&pb.RoomStatus{
			Game:   game,
			RoomId: roomId,
			Type:   roomType,
		},
	}
}

//Forward send room or player status to specified server
func (roomStatus *RoomStatusClient) Forward() error {
	conn, err := grpc.Dial("127.0.0.1:10000", grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewStatusClient(conn)
	response, err := client.SendRoomStatus(context.Background(), roomStatus.roomStatus)
	fmt.Println(response.Message)

	return nil
}
