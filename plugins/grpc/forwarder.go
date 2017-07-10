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
	"reflect"
	"strings"

	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/eventforwarder"
	pb "github.com/topfreegames/maestro/plugins/grpc/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

// GRPCForwarder struct
type GRPCForwarder struct {
	config        *viper.Viper
	client        pb.GRPCForwarderClient
	serverAddress string
}

// ForwarderFunc is the type of functions in GRPCForwarder
type ForwarderFunc func(client pb.GRPCForwarderClient, infos map[string]interface{}) (int32, error)

func (g *GRPCForwarder) roomStatusRequest(infos map[string]interface{}, status pb.RoomStatus_RoomStatusType) *pb.RoomStatus {
	req := &pb.RoomStatus{
		Room: &pb.Room{
			Game:   infos["game"].(string),
			RoomId: infos["roomId"].(string),
			Host:   infos["host"].(string),
			Port:   int32(infos["port"].(int)),
		},
		StatusType: status,
	}
	if meta, ok := infos["metadata"].(map[string]string); ok {
		req.Room.Metadata = meta
	}
	return req
}

func (g *GRPCForwarder) roomStatus(infos map[string]interface{}, roomStatus pb.RoomStatus_RoomStatusType) (status int32, err error) {
	req := g.roomStatusRequest(infos, roomStatus)
	response, err := g.client.SendRoomStatus(context.Background(), req)
	if err != nil {
		return 500, err
	}
	return response.Code, err
}

// Ready status
func (g *GRPCForwarder) Ready(infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(infos, pb.RoomStatus_ready)
}

// Occupied status
func (g *GRPCForwarder) Occupied(infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(infos, pb.RoomStatus_occupied)
}

// Terminating status
func (g *GRPCForwarder) Terminating(infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(infos, pb.RoomStatus_terminating)
}

// Terminated status
func (g *GRPCForwarder) Terminated(infos map[string]interface{}) (status int32, err error) {
	return g.roomStatus(infos, pb.RoomStatus_terminated)
}

//Forward send room or player status to specified server
func (g *GRPCForwarder) Forward(event string, infos map[string]interface{}) (status int32, err error) {
	f := reflect.ValueOf(g).MethodByName(strings.Title(event))
	if !f.IsValid() {
		return 500, fmt.Errorf("error calling method %s in plugin", event)
	}
	ret := f.Call([]reflect.Value{reflect.ValueOf(infos)})
	if _, ok := ret[1].Interface().(error); !ok {
		return ret[0].Interface().(int32), nil
	}
	return ret[0].Interface().(int32), ret[1].Interface().(error)
}

func (g *GRPCForwarder) configure() error {
	g.serverAddress = g.config.GetString("address")
	if g.serverAddress == "" {
		return fmt.Errorf("no grpc server address informed")
	}
	conn, err := grpc.Dial(g.serverAddress, grpc.WithInsecure())
	if err != nil {
		return err
	}
	g.client = pb.NewGRPCForwarderClient(conn)
	return nil
}

// NewForwarder returns a new GRPCForwarder
func NewForwarder(config *viper.Viper) (eventforwarder.EventForwarder, error) {
	g := &GRPCForwarder{
		config: config,
	}
	err := g.configure()
	if err != nil {
		return nil, err
	}
	return g, nil
}
