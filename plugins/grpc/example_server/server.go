// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import (
	"fmt"
	pb "github.com/topfreegames/maestro/plugins/grpc/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"net"
)

type server struct{}

func (*server) SendRoomStatus(ctx context.Context, roomStatus *pb.RoomStatus) (*pb.Response, error) {
	fmt.Println("Received msg", roomStatus.GetRoom(), roomStatus.GetStatusType())
	return &pb.Response{
		Message: "Hi!",
		Code:    200,
	}, nil
}

func (*server) SendPlayerEvent(ctx context.Context, playerEvent *pb.PlayerEvent) (*pb.Response, error) {
	fmt.Println("Received msg", playerEvent.GetRoom(), playerEvent.GetPlayerId(), playerEvent.GetEventType())
	return &pb.Response{
		Message: "Hi!",
		Code:    200,
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":10000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	fmt.Println("Server listening at :10000")
	grpcServer := grpc.NewServer()
	pb.RegisterGRPCForwarderServer(grpcServer, &server{})

	grpcServer.Serve(lis)
}
