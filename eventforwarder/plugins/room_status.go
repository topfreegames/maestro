// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package eventforwarder

import "C"

import (
	pb "github.com/topfreegames/maestro/eventforwarder/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

//RoomStatusClient implements EventForwarder interface
type RoomStatusClient struct {
	RoomStatus    *pb.Room
	ServerAddress string
}

//NewRoomStatus is the RoomStatusClient constructor
func NewRoomStatus(serverAddress string) *RoomStatusClient {
	return &RoomStatusClient{
		ServerAddress: serverAddress,
	}
}

func (roomStatus *RoomStatusClient) setup(infos map[string]interface{}) {
	roomStatus.RoomStatus = &pb.Room{
		Game:     infos["game"].(string),
		RoomId:   infos["roomId"].(string),
		RoomType: infos["roomType"].(string),
		Status: &pb.Status{
			infos["status"].(string),
		},
	}
}

//Forward send room or player status to specified server
func (roomStatus *RoomStatusClient) Forward(infos map[string]interface{}) (*pb.Response, error) {
	conn, err := grpc.Dial(roomStatus.ServerAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewStatusServiceClient(conn)
	roomStatus.setup(infos)
	response, err := client.SendStatus(context.Background(), roomStatus.RoomStatus)

	return response, err
}
