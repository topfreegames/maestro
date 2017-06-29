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

func (*server) SendRoomStatus(ctx context.Context, status *pb.RoomStatus) (*pb.Response, error) {
	fmt.Println("Received msg", status.GetGame(), status.GetType(), status.GetRoomId())
	return &pb.Response{"Hi!"}, nil
}

func (*server) SendPlayerStatus(ctx context.Context, status *pb.PlayerStatus) (*pb.Response, error) {
	fmt.Println("Received msg", status.GetGame(), status.GetType(), status.GetRoomId(), status.GetUserId())
	return &pb.Response{"Hi!"}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":10000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	fmt.Println("Server listening at :10000")
	grpcServer := grpc.NewServer()
	pb.RegisterStatusServer(grpcServer, &server{})

	grpcServer.Serve(lis)
}
