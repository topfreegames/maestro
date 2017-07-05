// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import "C"

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/eventforwarder"
	pb "github.com/topfreegames/maestro/plugins/grpc/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

// GRPCForwarder struct
type GRPCForwarder struct {
	methodMap map[string]ForwarderFunc
}

// ForwarderFunc is the type of functions in GRPCForwarder
type ForwarderFunc func(client pb.GRPCForwarderClient, infos map[string]interface{}) (int32, error)

func (g *GRPCForwarder) roomStatusRequest(infos map[string]interface{}, status pb.RoomStatus_RoomStatusType) *pb.RoomStatus {
	return &pb.RoomStatus{
		Room: &pb.Room{
			Game:     infos["game"].(string),
			RoomId:   infos["roomId"].(string),
			RoomType: infos["roomType"].(string),
			Host:     infos["host"].(string),
			Port:     int32(infos["port"].(int)),
		},
		StatusType: status,
	}
}

func (g *GRPCForwarder) roomStatus(client pb.GRPCForwarderClient, infos map[string]interface{}, roomStatus pb.RoomStatus_RoomStatusType) (status int32, err error) {
	req := g.roomStatusRequest(infos, roomStatus)
	response, err := client.SendRoomStatus(context.Background(), req)
	if err != nil {
		return 500, err
	}
	return response.Code, err
}

func (g *GRPCForwarder) roomReady(client pb.GRPCForwarderClient, infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(client, infos, pb.RoomStatus_ROOM_READY)
}

func (g *GRPCForwarder) roomOccupied(client pb.GRPCForwarderClient, infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(client, infos, pb.RoomStatus_ROOM_OCCUPIED)
}

func (g *GRPCForwarder) roomTerminating(client pb.GRPCForwarderClient, infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(client, infos, pb.RoomStatus_ROOM_TERMINATING)
}

func (g *GRPCForwarder) roomTerminated(client pb.GRPCForwarderClient, infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(client, infos, pb.RoomStatus_ROOM_TERMINATED)
}

//Forward send room or player status to specified server
func (g *GRPCForwarder) Forward(config *viper.Viper, event string, infos map[string]interface{}) (status int32, err error) {
	conn, err := grpc.Dial(config.GetString("forwarders.grpc.address"), grpc.WithInsecure())
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	client := pb.NewGRPCForwarderClient(conn)
	method, ok := g.methodMap[event]
	if !ok {
		return 404, fmt.Errorf("method %s not found", event)
	}
	return method(client, infos)
}

// NewForwarder returns a new GRPCForwarder
func NewForwarder() eventforwarder.EventForwarder {
	g := &GRPCForwarder{}
	g.methodMap = map[string]ForwarderFunc{
		"roomReady":       g.roomReady,
		"roomOccupied":    g.roomOccupied,
		"roomTerminating": g.roomTerminating,
		"roomTerminated":  g.roomTerminated,
	}
	return g
}
