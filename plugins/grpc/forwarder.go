// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/eventforwarder"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

// GRPCForwarder struct
type GRPCForwarder struct {
	config        *viper.Viper
	client        pb.GRPCForwarderClient
	logger        log.FieldLogger
	serverAddress string
}

// ForwarderFunc is the type of functions in GRPCForwarder
type ForwarderFunc func(client pb.GRPCForwarderClient, infos map[string]interface{}) (int32, error)

func (g *GRPCForwarder) roomStatusRequest(infos map[string]interface{}, status pb.RoomStatus_RoomStatusType) *pb.RoomStatus {
	game := infos["game"].(string)
	roomID := infos["roomId"].(string)
	host := infos["host"].(string)
	port := infos["port"].(int32)

	g.logger.WithFields(log.Fields{
		"op":     "roomStatusRequest",
		"game":   game,
		"roomId": roomID,
		"host":   host,
		"port":   port,
	}).Debug("getting room status request")

	req := &pb.RoomStatus{
		Room: &pb.Room{
			Game:   game,
			RoomId: roomID,
			Host:   host,
			Port:   port,
		},
		StatusType: status,
	}
	if meta, ok := infos["metadata"].(map[string]interface{}); ok {
		m := make(map[string]string)
		for key, value := range meta {
			m[key] = value.(string)
		}
		req.Room.Metadata = m
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

func (g *GRPCForwarder) playerEventRequest(infos map[string]interface{}, event pb.PlayerEvent_PlayerEventType) *pb.PlayerEvent {
	m := make(map[string]string)
	for key, value := range infos {
		m[key] = value.(string)
	}
	req := &pb.PlayerEvent{
		PlayerId: infos["playerId"].(string),
		Room: &pb.Room{
			RoomId: infos["roomId"].(string),
		},
		EventType: event,
		Metadata:  m,
	}

	return req
}

func (g *GRPCForwarder) playerEvent(infos map[string]interface{}, playerEvent pb.PlayerEvent_PlayerEventType) (status int32, err error) {
	_, ok := infos["playerId"].(string)
	if !ok {
		return 500, errors.New("no playerId specified in metadata")
	}
	_, ok = infos["roomId"].(string)
	if !ok {
		return 500, errors.New("no roomId specified in metadata")
	}
	req := g.playerEventRequest(infos, playerEvent)
	response, err := g.client.SendPlayerEvent(context.Background(), req)
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

// PlayerJoin status
func (g *GRPCForwarder) PlayerJoin(infos map[string]interface{}) (status int32, err error) {
	return g.playerEvent(infos, pb.PlayerEvent_PLAYER_JOINED)
}

// PlayerLeft status
func (g *GRPCForwarder) PlayerLeft(infos map[string]interface{}) (status int32, err error) {
	return g.playerEvent(infos, pb.PlayerEvent_PLAYER_LEFT)
}

//Forward send room or player status to specified server
func (g *GRPCForwarder) Forward(event string, infos map[string]interface{}) (status int32, err error) {
	l := g.logger.WithFields(log.Fields{
		"op":         "Forward",
		"event":      event,
		"infos":      infos,
		"serverAddr": g.serverAddress,
	})
	l.Debug("forwarding event")
	f := reflect.ValueOf(g).MethodByName(strings.Title(event))
	if !f.IsValid() {
		return 500, fmt.Errorf("error calling method %s in plugin", event)
	}
	ret := f.Call([]reflect.Value{reflect.ValueOf(infos)})
	if _, ok := ret[1].Interface().(error); !ok {
		return ret[0].Interface().(int32), nil
	}
	l.Debug("successfully forwarded event")
	return ret[0].Interface().(int32), ret[1].Interface().(error)
}

func (g *GRPCForwarder) configure() error {
	l := g.logger.WithFields(log.Fields{
		"op": "configure",
	})
	g.serverAddress = g.config.GetString("address")
	if g.serverAddress == "" {
		return fmt.Errorf("no grpc server address informed")
	}
	l.Infof("connecting to grpc server at: %s", g.serverAddress)
	conn, err := grpc.Dial(g.serverAddress, grpc.WithInsecure())
	if err != nil {
		return err
	}
	g.client = pb.NewGRPCForwarderClient(conn)
	return nil
}

// NewForwarder returns a new GRPCForwarder
func NewForwarder(config *viper.Viper, logger log.FieldLogger) (eventforwarder.EventForwarder, error) {
	g := &GRPCForwarder{
		config: config,
	}
	g.logger = logger
	err := g.configure()
	if err != nil {
		return nil, err
	}
	return g, nil
}
