// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package main

import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/eventforwarder"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
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
type ForwarderFunc func(client pb.GRPCForwarderClient, infos, fwdMetadata map[string]interface{}) (int32, string, error)

func (g *GRPCForwarder) roomResync(ctx context.Context, infos map[string]interface{}, roomStatus pb.RoomStatus_RoomStatusType) (status int32, message string, err error) {
	req := g.roomStatusRequest(infos, roomStatus)

	ctx, cancel := context.WithTimeout(ctx, g.config.GetDuration("timeout"))
	defer cancel()

	response, err := g.client.SendRoomResync(ctx, req)
	if err != nil {
		return 500, "", err
	}
	return response.Code, response.Message, err
}

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
			if v, ok := value.(string); ok {
				m[key] = v
			} else {
				m[key] = fmt.Sprintf("%v", value)
			}
		}
		req.Room.Metadata = m

		if roomType, ok := meta["roomType"].(string); ok {
			req.Room.RoomType = roomType
			delete(meta, "roomType")
		}
	}
	return req
}

func (g *GRPCForwarder) roomStatus(ctx context.Context, infos map[string]interface{}, roomStatus pb.RoomStatus_RoomStatusType) (status int32, message string, err error) {
	req := g.roomStatusRequest(infos, roomStatus)

	ctx, cancel := context.WithTimeout(ctx, g.config.GetDuration("timeout"))
	defer cancel()

	response, err := g.client.SendRoomStatus(ctx, req)
	if err != nil {
		return 500, "", err
	}
	return response.Code, response.Message, err
}

func (g *GRPCForwarder) roomEventRequest(infos map[string]interface{}, eventType string) *pb.RoomEvent {
	game := infos["game"].(string)
	roomID := infos["roomId"].(string)
	host := infos["host"].(string)
	port := infos["port"].(int32)

	g.logger.WithFields(log.Fields{
		"op":     "roomEventRequest",
		"game":   game,
		"roomId": roomID,
		"host":   host,
		"port":   port,
		"event":  eventType,
	}).Debug("getting room event request")

	req := &pb.RoomEvent{
		Room: &pb.Room{
			Game:   game,
			RoomId: roomID,
			Host:   host,
			Port:   port,
		},
		EventType: eventType,
	}
	if meta, ok := infos["metadata"].(map[string]interface{}); ok {
		m := make(map[string]string)
		for key, value := range meta {
			if v, ok := value.(string); ok {
				m[key] = v
			} else {
				m[key] = fmt.Sprintf("%v", value)
			}
		}
		req.Room.Metadata = m
	}
	return req
}

func (g *GRPCForwarder) sendRoomEvent(ctx context.Context, infos map[string]interface{}, eventType string) (status int32, message string, err error) {
	req := g.roomEventRequest(infos, eventType)

	ctx, cancel := context.WithTimeout(ctx, g.config.GetDuration("timeout"))
	defer cancel()

	response, err := g.client.SendRoomEvent(ctx, req)
	if err != nil {
		return 500, "", err
	}
	return response.Code, response.Message, err
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

func (g *GRPCForwarder) playerEvent(ctx context.Context, infos map[string]interface{}, playerEvent pb.PlayerEvent_PlayerEventType) (status int32, message string, err error) {
	_, ok := infos["playerId"].(string)
	if !ok {
		return 500, "", errors.New("no playerId specified in metadata")
	}
	_, ok = infos["roomId"].(string)
	if !ok {
		return 500, "", errors.New("no roomId specified in metadata")
	}
	req := g.playerEventRequest(infos, playerEvent)

	ctx, cancel := context.WithTimeout(ctx, g.config.GetDuration("timeout"))
	defer cancel()

	response, err := g.client.SendPlayerEvent(ctx, req)
	if err != nil {
		return 500, "", err
	}
	return response.Code, response.Message, err
}

func (g *GRPCForwarder) roomInfoRequest(infos map[string]interface{}) (*pb.RoomInfo, error) {
	game := infos["game"].(string)

	g.logger.WithFields(log.Fields{
		"op":   "roomInfoRequest",
		"game": game,
	}).Debug("getting room info request")

	var m map[string]interface{}
	var ok bool
	if infos["metadata"] != nil {
		if m, ok = infos["metadata"].(map[string]interface{}); !ok {
			var n map[interface{}]interface{}
			if n, ok = infos["metadata"].(map[interface{}]interface{}); ok {
				m = map[string]interface{}{}
				for key, value := range n {
					switch key := key.(type) {
					case string:
						m[key] = value
					}
				}
			} else {
				g.logger.WithFields(log.Fields{
					"op":       "roomInfoRequest",
					"game":     game,
					"metadata": fmt.Sprintf("%T", infos["metadata"]),
				}).Warn("invalid metadata provided")
			}
		}
		delete(infos, "metadata")
	}
	infosBytes, err := json.Marshal(infos)
	if err != nil {
		return nil, err
	}

	var req pb.RoomInfo
	err = json.Unmarshal(infosBytes, &req)
	if err != nil {
		return nil, err
	}

	if m != nil {
		req.Metadata = make(map[string]string)

		for key, value := range m {
			switch value := value.(type) {
			case string:
				req.Metadata[key] = value
			default:
				req.Metadata[key] = fmt.Sprintf("%v", value)
			}
		}
	}
	return &req, nil
}

func (g *GRPCForwarder) sendRoomInfo(ctx context.Context, infos map[string]interface{}) (status int32, message string, err error) {
	req, err := g.roomInfoRequest(infos)
	if err != nil {
		return 500, "", err
	}

	ctx, cancel := context.WithTimeout(ctx, g.config.GetDuration("timeout"))
	defer cancel()

	response, err := g.client.SendRoomInfo(ctx, req)
	if err != nil {
		return 500, "", err
	}
	return response.Code, response.Message, err
}

// Ready status
func (g *GRPCForwarder) Ready(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomStatus(ctx, infos, pb.RoomStatus_ready)
}

// Occupied status
func (g *GRPCForwarder) Occupied(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomStatus(ctx, infos, pb.RoomStatus_occupied)
}

// Terminating status
func (g *GRPCForwarder) Terminating(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomStatus(ctx, infos, pb.RoomStatus_terminating)
}

// Terminated status
func (g *GRPCForwarder) Terminated(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomStatus(ctx, infos, pb.RoomStatus_terminated)
}

// PingReady status
func (g *GRPCForwarder) PingReady(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomResync(ctx, infos, pb.RoomStatus_ready)
}

// PingOccupied status
func (g *GRPCForwarder) PingOccupied(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomResync(ctx, infos, pb.RoomStatus_occupied)
}

// PingTerminating status
func (g *GRPCForwarder) PingTerminating(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomResync(ctx, infos, pb.RoomStatus_terminating)
}

// PingTerminated status
func (g *GRPCForwarder) PingTerminated(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	return g.roomResync(ctx, infos, pb.RoomStatus_terminated)
}

// PlayerJoin event
func (g *GRPCForwarder) PlayerJoin(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergePlayerInfos(infos, fwdMetadata)
	return g.playerEvent(ctx, infos, pb.PlayerEvent_PLAYER_JOINED)
}

// PlayerLeft event
func (g *GRPCForwarder) PlayerLeft(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergePlayerInfos(infos, fwdMetadata)
	return g.playerEvent(ctx, infos, pb.PlayerEvent_PLAYER_LEFT)
}

// RoomEvent sends a generic room event
func (g *GRPCForwarder) RoomEvent(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	infos = g.mergeInfos(infos, fwdMetadata)
	eventType := infos["metadata"].(map[string]interface{})["eventType"].(string)
	delete(infos["metadata"].(map[string]interface{}), "eventType")
	return g.sendRoomEvent(ctx, infos, eventType)
}

// SchedulerEvent sends a scheduler event
func (g *GRPCForwarder) SchedulerEvent(ctx context.Context, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	for k, v := range fwdMetadata {
		infos[k] = v
	}
	return g.sendRoomInfo(ctx, infos)
}

//Forward send room or player status to specified server
func (g *GRPCForwarder) Forward(ctx context.Context, event string, infos, fwdMetadata map[string]interface{}) (status int32, message string, err error) {
	l := g.logger.WithFields(log.Fields{
		"op":          "Forward",
		"source":      "plugin/grpc",
		"event":       event,
		"infos":       fmt.Sprintf("%v", infos),
		"fwdMetadata": fmt.Sprintf("%v", fwdMetadata),
		"serverAddr":  g.serverAddress,
	})
	l.Info("forwarding event")
	f := reflect.ValueOf(g).MethodByName(strings.Title(event))
	if !f.IsValid() {
		return 500, "", fmt.Errorf("error calling method %s in plugin", event)
	}
	ret := f.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(infos), reflect.ValueOf(fwdMetadata)})
	err, ok := ret[2].Interface().(error)
	if ok {
		l.WithError(err).Error("forward event failed")
		return ret[0].Interface().(int32), ret[1].Interface().(string), ret[2].Interface().(error)
	}
	l.Info("successfully forwarded event")
	return ret[0].Interface().(int32), ret[1].Interface().(string), nil
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
	tracer := opentracing.GlobalTracer()
	conn, err := grpc.Dial(
		g.serverAddress,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	)
	if err != nil {
		return err
	}
	g.client = pb.NewGRPCForwarderClient(conn)
	return nil
}

func (g *GRPCForwarder) mergeInfos(infos, fwdMetadata map[string]interface{}) map[string]interface{} {
	if fwdMetadata != nil {
		if roomType, ok := fwdMetadata["roomType"]; ok {
			if metadata, ok := infos["metadata"].(map[string]interface{}); ok {
				if metadata != nil {
					metadata["roomType"] = roomType
				} else {
					infos["metadata"] = map[string]interface{}{"roomType": roomType}
				}
			} else if metadata, ok := infos["metadata"].(map[interface{}]interface{}); ok {
				if metadata != nil {
					metadata["roomType"] = roomType
				} else {
					infos["metadata"] = map[string]interface{}{"roomType": roomType}
				}
			} else if infos["metadata"] == nil {
				infos["metadata"] = map[string]interface{}{"roomType": roomType}
			} else {
				g.logger.WithFields(log.Fields{
					"op":       "mergeInfos",
					"metadata": fmt.Sprintf("%T", infos["metadata"]),
				}).Warn("invalid metadata provided")
			}
		}
	}
	return infos
}

func (g *GRPCForwarder) mergePlayerInfos(infos, fwdMetadata map[string]interface{}) map[string]interface{} {
	if fwdMetadata != nil {
		if roomType, ok := fwdMetadata["roomType"]; ok {
			if infos != nil {
				infos["roomType"] = roomType
			} else {
				infos = map[string]interface{}{"roomType": roomType}
			}
		}
	}
	return infos
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
