// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package eventforwarder

import (
	pb "github.com/topfreegames/maestro/eventforwarder/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

type RoomStatusClient struct {
	RoomStatus    *pb.Room
	ServerAddress string
}

func NewRoomStatus(game, roomId, roomType, status, serverAddress string) *RoomStatusClient {
	return &RoomStatusClient{
		RoomStatus: &pb.Room{
			Game:     game,
			RoomId:   roomId,
			RoomType: roomType,
			Status:   &pb.Status{status},
		},
		ServerAddress: serverAddress,
	}
}

//Forward send room or player status to specified server
func (roomStatus *RoomStatusClient) Forward() (*pb.Response, error) {
	conn, err := grpc.Dial(roomStatus.ServerAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewStatusServiceClient(conn)
	response, err := client.SendRoomStatus(context.Background(), roomStatus.RoomStatus)

	return response, err
}
