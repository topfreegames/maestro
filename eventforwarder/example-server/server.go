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
	"log"
	"net"
)

type server struct{}

func (*server) SendRoomStatus(ctx context.Context, room *pb.Room) (*pb.Response, error) {
	fmt.Println("Received msg", room.GetGame(), room.GetRoomType(), room.GetRoomId())
	return &pb.Response{"Hi!"}, nil
}

func (*server) SendPlayerStatus(ctx context.Context, player *pb.Player) (*pb.Response, error) {
	fmt.Println("Received msg", player.GetGame(), player.GetRoomType(), player.GetRoomId(), player.GetUserId())
	return &pb.Response{"Hi!"}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":10000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	fmt.Println("Server listening at :10000")
	grpcServer := grpc.NewServer()
	pb.RegisterStatusServiceServer(grpcServer, &server{})

	grpcServer.Serve(lis)
}
